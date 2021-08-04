// Copyright 2021 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	cerbos "github.com/cerbos/cerbos/client"
	"github.com/cerbos/demo-rest/db"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

type authCtxKeyType struct{}

var authCtxKey = authCtxKeyType{}

type authContext struct {
	username  string
	principal *cerbos.Principal
}

const (
	inventoryResource = "inventory"
	orderResource     = "order"
)

// toOrderResource creates a Cerbos resource from the given order.
func toOrderResource(o db.Order) *cerbos.Resource {
	return cerbos.NewResource(orderResource, strconv.FormatUint(o.ID, 10)).
		WithAttr("items", o.Items).
		WithAttr("status", o.Status).
		WithAttr("owner", o.Owner)
}

// toInventoryResource creates a Cerbos resource from the given inventory record.
func toInventoryResource(i db.InventoryRecord) *cerbos.Resource {
	return cerbos.NewResource(inventoryResource, i.ID).
		WithAttr("aisle", i.Aisle).
		WithAttr("price", i.Price).
		WithAttr("quantity", i.Quantity)
}

// Service implements the store API.
type Service struct {
	cerbos    cerbos.Client
	orders    *db.OrderDB
	inventory *db.Inventory
}

func New(cerbosAddr string) (*Service, error) {
	c, err := cerbos.New(cerbosAddr, cerbos.WithPlaintext())
	if err != nil {
		return nil, err
	}

	return &Service{cerbos: c, orders: db.NewOrderDB(), inventory: db.NewInventory()}, nil
}

func (s *Service) Handler() http.Handler {
	authn := authenticationMiddleware

	r := mux.NewRouter()
	r.Use(authn)

	r.HandleFunc("/store/order", s.handleOrderCreate).Methods(http.MethodPut)
	r.HandleFunc("/store/order/{orderID}", s.handleOrderUpdate).Methods(http.MethodPost)
	r.HandleFunc("/store/order/{orderID}", s.handleOrderDelete).Methods(http.MethodDelete)
	r.HandleFunc("/store/order/{orderID}", s.handleOrderView).Methods(http.MethodGet)

	r.HandleFunc("/backoffice/order/{orderID}/status/{status}", s.handleBackofficeOrderUpdate).Methods(http.MethodPost)

	r.HandleFunc("/backoffice/inventory", s.handleInventoryAdd).Methods(http.MethodPut)
	r.HandleFunc("/backoffice/inventory/{itemID}", s.handleInventoryUpdate).Methods(http.MethodPost)
	r.HandleFunc("/backoffice/inventory/{itemID}", s.handleInventoryDelete).Methods(http.MethodDelete)
	r.HandleFunc("/backoffice/inventory/{itemID}", s.handleInventoryGet).Methods(http.MethodGet)
	r.HandleFunc("/backoffice/inventory/{itemID}/pick/{quantity}", s.handleInventoryPick).Methods(http.MethodPost)
	r.HandleFunc("/backoffice/inventory/{itemID}/replenish/{quantity}", s.handleInventoryReplenish).Methods(http.MethodPost)

	r.HandleFunc("/health", s.handleHealth)

	return handlers.LoggingHandler(log.Writer(), r)
}

// authenticationMiddleware handles the verification of username and password,
// creates a Cerbos principal and adds it to the request context.
func authenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the basic auth credentials from the request.
		user, password, ok := r.BasicAuth()
		if ok {
			// check the password and retrieve the auth context.
			authCtx, err := buildAuthContext(user, password, r)
			if err != nil {
				log.Printf("Failed to authenticate user [%s]: %v", user, err)
			} else {
				// Add the retrieved principal to the context.
				ctx := context.WithValue(r.Context(), authCtxKey, authCtx)
				next.ServeHTTP(w, r.WithContext(ctx))

				return
			}
		}

		// No credentials provided or the credentials are invalid.
		w.Header().Set("WWW-Authenticate", `Basic realm="auth"`)
		writeMessage(w, http.StatusUnauthorized, "Authentication required")
	})
}

// buildAuthContext verifies the username and password and returns a new authContext object.
func buildAuthContext(username, password string, r *http.Request) (*authContext, error) {
	// Lookup the user from the database.
	record, err := db.LookupUser(r.Context(), username)
	if err != nil {
		return nil, err
	}

	// Check that the password matches.
	if err := bcrypt.CompareHashAndPassword(record.PasswordHash, []byte(password)); err != nil {
		return nil, err
	}

	// Create a new principal object with information from the database and the request.
	principal := cerbos.NewPrincipal(username).
		WithRoles(record.Roles...).
		WithAttr("aisles", record.Aisles).
		WithAttr("ipAddress", r.RemoteAddr)

	return &authContext{username: username, principal: principal}, nil
}

// isAllowed is a utility function to check each action against a Cerbos policy.
func (s *Service) isAllowed(ctx context.Context, resource *cerbos.Resource, action string) bool {
	authCtx := getAuthContext(ctx)
	if authCtx == nil {
		return false
	}

	allowed, err := s.cerbos.IsAllowed(ctx, authCtx.principal, resource, action)
	if err != nil {
		log.Printf("ERROR: %v", err)
		return false
	}

	return allowed
}

// getAuthContext retrieves the principal stored in the context by the authentication middleware.
func getAuthContext(ctx context.Context) *authContext {
	ac := ctx.Value(authCtxKey)
	if ac == nil {
		return nil
	}

	return ac.(*authContext)
}

func (s *Service) handleOrderCreate(w http.ResponseWriter, r *http.Request) {
	defer cleanup(r)

	order, err := readOrder(r.Body)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusBadRequest, "Bad request")
		return
	}

	resource := cerbos.NewResource(orderResource, "new").WithAttr("items", order.Items)
	if !s.isAllowed(r.Context(), resource, "CREATE") {
		writeMessage(w, http.StatusForbidden, "Operation not allowed")
		return
	}

	authCtx := getAuthContext(r.Context())
	orderID := s.orders.Create(authCtx.username, order)

	writeJSON(w, http.StatusCreated, struct {
		OrderID uint64 `json:"orderID"`
	}{OrderID: orderID})
}

func (s *Service) handleOrderUpdate(w http.ResponseWriter, r *http.Request) {
	defer cleanup(r)

	order, err := s.retrieveOrder(r)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusBadRequest, "Order not found")
		return
	}

	if !s.isAllowed(r.Context(), toOrderResource(order), "UPDATE") {
		writeMessage(w, http.StatusForbidden, "Operation not allowed")
		return
	}

	newOrder, err := readOrder(r.Body)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusBadRequest, "Bad request")
		return
	}

	if err := s.orders.Update(order.ID, newOrder); err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusInternalServerError, "Failed to update order")
		return
	}

	writeMessage(w, http.StatusOK, "Order updated")
}

func (s *Service) handleOrderDelete(w http.ResponseWriter, r *http.Request) {
	defer cleanup(r)

	order, err := s.retrieveOrder(r)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusBadRequest, "Order not found")
		return
	}

	if !s.isAllowed(r.Context(), toOrderResource(order), "DELETE") {
		writeMessage(w, http.StatusForbidden, "Operation not allowed")
		return
	}

	if err := s.orders.Delete(order.ID); err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusInternalServerError, "Failed to delete order")
		return
	}

	writeMessage(w, http.StatusOK, "Order cancelled")
}

func (s *Service) handleOrderView(w http.ResponseWriter, r *http.Request) {
	defer cleanup(r)

	order, err := s.retrieveOrder(r)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusBadRequest, "Order not found")
		return
	}

	if !s.isAllowed(r.Context(), toOrderResource(order), "VIEW") {
		writeMessage(w, http.StatusForbidden, "Operation not allowed")
		return
	}

	writeJSON(w, http.StatusOK, order)
}

func (s *Service) handleBackofficeOrderUpdate(w http.ResponseWriter, r *http.Request) {
	defer cleanup(r)

	order, err := s.retrieveOrder(r)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusBadRequest, "Order not found")
		return
	}

	vars := mux.Vars(r)
	status := vars["status"]

	resource := toOrderResource(order).WithAttr("newStatus", status)
	if !s.isAllowed(r.Context(), resource, "UPDATE_STATUS") {
		writeMessage(w, http.StatusForbidden, "Operation not allowed")
		return
	}

	if err := s.orders.SetStatus(order.ID, status); err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusInternalServerError, "Failed to update order")
		return
	}

	writeMessage(w, http.StatusOK, "Order status updated")
}

func (s *Service) retrieveOrder(r *http.Request) (db.Order, error) {
	vars := mux.Vars(r)

	orderID, err := strconv.ParseUint(vars["orderID"], 10, 64)
	if err != nil {
		return db.Order{}, err
	}

	return s.orders.Get(orderID)
}

func (s *Service) handleInventoryAdd(w http.ResponseWriter, r *http.Request) {
	defer cleanup(r)

	item, err := readInventoryItem(r.Body)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusBadRequest, "Bad request")
		return
	}

	resource := cerbos.NewResource(inventoryResource, "new").WithAttr("aisle", item.Aisle)
	if !s.isAllowed(r.Context(), resource, "CREATE") {
		writeMessage(w, http.StatusForbidden, "Operation not allowed")
		return
	}

	if err := s.inventory.Add(item); err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusBadRequest, "Bad request")
		return
	}

	writeMessage(w, http.StatusCreated, "Item added")
}

func (s *Service) handleInventoryUpdate(w http.ResponseWriter, r *http.Request) {
	defer cleanup(r)

	record, err := s.retrieveInventoryRecord(r)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusBadRequest, "No such item")
		return
	}

	item, err := readInventoryItem(r.Body)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusBadRequest, "Bad request")
		return
	}

	resource := toInventoryResource(record).WithAttr("newAisle", item.Aisle).WithAttr("newPrice", item.Price)
	if !s.isAllowed(r.Context(), resource, "UPDATE") {
		writeMessage(w, http.StatusForbidden, "Operation not allowed")
		return
	}

	if err := s.inventory.Update(item); err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusInternalServerError, "Failed to update item")
		return
	}

	writeMessage(w, http.StatusOK, "Item updated")
}

func (s *Service) handleInventoryDelete(w http.ResponseWriter, r *http.Request) {
	defer cleanup(r)

	record, err := s.retrieveInventoryRecord(r)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusBadRequest, "No such item")
		return
	}

	resource := toInventoryResource(record)
	if !s.isAllowed(r.Context(), resource, "DELETE") {
		writeMessage(w, http.StatusForbidden, "Operation not allowed")
		return
	}

	if err := s.inventory.Delete(record.ID); err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusInternalServerError, "Failed to delete item")
		return
	}

	writeMessage(w, http.StatusOK, "Item deleted")
}

func (s *Service) handleInventoryGet(w http.ResponseWriter, r *http.Request) {
	defer cleanup(r)

	record, err := s.retrieveInventoryRecord(r)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusBadRequest, "No such item")
		return
	}

	resource := toInventoryResource(record)
	if !s.isAllowed(r.Context(), resource, "VIEW") {
		writeMessage(w, http.StatusForbidden, "Operation not allowed")
		return
	}

	writeJSON(w, http.StatusOK, record)
}

func (s *Service) handleInventoryPick(w http.ResponseWriter, r *http.Request) {
	defer cleanup(r)

	record, err := s.retrieveInventoryRecord(r)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusBadRequest, "No such item")
		return
	}

	vars := mux.Vars(r)

	pickQty, err := strconv.Atoi(vars["quantity"])
	if err != nil || pickQty < 1 {
		writeMessage(w, http.StatusBadRequest, "Invalid quantity")
		return
	}

	resource := toInventoryResource(record).WithAttr("pickQuantity", pickQty)
	if !s.isAllowed(r.Context(), resource, "PICK") {
		writeMessage(w, http.StatusForbidden, "Operation not allowed")
		return
	}

	newQty, err := s.inventory.UpdateQuantity(record.ID, -pickQty)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusInternalServerError, "Failed to update item")
		return
	}

	writeJSON(w, http.StatusOK, struct {
		NewQuantity int `json:"newQuantity"`
	}{NewQuantity: newQty})
}

func (s *Service) handleInventoryReplenish(w http.ResponseWriter, r *http.Request) {
	defer cleanup(r)

	record, err := s.retrieveInventoryRecord(r)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusBadRequest, "No such item")
		return
	}

	vars := mux.Vars(r)

	qty, err := strconv.Atoi(vars["quantity"])
	if err != nil || qty < 1 {
		writeMessage(w, http.StatusBadRequest, "Invalid quantity")
		return
	}

	resource := toInventoryResource(record).WithAttr("newQuantity", qty)
	if !s.isAllowed(r.Context(), resource, "REPLENISH") {
		writeMessage(w, http.StatusForbidden, "Operation not allowed")
		return
	}

	newQty, err := s.inventory.UpdateQuantity(record.ID, qty)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusInternalServerError, "Failed to update item")
		return
	}

	writeJSON(w, http.StatusOK, struct {
		NewQuantity int `json:"newQuantity"`
	}{NewQuantity: newQty})
}

func (s *Service) retrieveInventoryRecord(r *http.Request) (db.InventoryRecord, error) {
	vars := mux.Vars(r)

	return s.inventory.GetItem(vars["itemID"])
}

func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	defer cleanup(r)

	fmt.Fprintln(w, "OK")
}

func cleanup(r *http.Request) {
	if r.Body != nil {
		_, _ = io.Copy(io.Discard, r.Body)
		r.Body.Close()
	}
}

func readOrder(r io.Reader) (db.CustomerOrder, error) {
	dec := json.NewDecoder(r)

	var order db.CustomerOrder
	err := dec.Decode(&order)

	return order, err
}

func readInventoryItem(r io.Reader) (db.InventoryItem, error) {
	dec := json.NewDecoder(r)

	var item db.InventoryItem
	err := dec.Decode(&item)

	return item, err
}

type genericResponse struct {
	Message string `json:"message"`
}

func writeMessage(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, genericResponse{Message: msg})
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	_ = enc.Encode(v)
}
