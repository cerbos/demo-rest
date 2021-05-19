// Copyright 2021 Zenauth Ltd.

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

type principalKeyType struct{}

var principalKey = principalKeyType{}

type Service struct {
	cerbos cerbos.Client
	orders *db.OrderDB
}

func New(cerbosAddr string) (*Service, error) {
	c, err := cerbos.New(cerbosAddr, cerbos.WithPlaintext())
	if err != nil {
		return nil, err
	}

	return &Service{cerbos: c, orders: db.NewOrderDB()}, nil
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

	r.HandleFunc("/health", s.handleHealth)

	return handlers.LoggingHandler(log.Writer(), r)
}

func authenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the basic auth credentials from the request.
		user, password, ok := r.BasicAuth()
		if ok {
			// check the password and retrieve the user record.
			principal, err := retrievePrincipal(user, password, r)
			if err != nil {
				log.Printf("Failed to authenticate user [%s]: %v", user, err)
			} else {
				// Add the retrieved principal to the context.
				ctx := context.WithValue(r.Context(), principalKey, principal)
				next.ServeHTTP(w, r.WithContext(ctx))

				return
			}
		}

		// No credentials provided or the credentials are invalid.
		w.Header().Set("WWW-Authenticate", `Basic realm="auth"`)
		writeMessage(w, http.StatusUnauthorized, "Authentication required")
	})
}

// retrievePrincipal verifies the username and password and returns a new principal object.
func retrievePrincipal(username, password string, r *http.Request) (*cerbos.Principal, error) {
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
		WithAttr("ipAddress", r.RemoteAddr)

	return principal, nil
}

func (s *Service) handleOrderCreate(w http.ResponseWriter, r *http.Request) {
	defer cleanup(r)

	order, err := readOrder(r.Body)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusBadRequest, "Bad request")
		return
	}

	resource := cerbos.NewResource("order", "new").WithAttr("items", order.Items)
	if !s.isAllowed(r.Context(), resource, "CREATE") {
		writeMessage(w, http.StatusForbidden, "Operation not allowed")
		return
	}

	principal := getPrincipal(r.Context())
	orderID := s.orders.Create(principal.Id, order)

	writeJSON(w, http.StatusCreated, struct {
		OrderID uint64 `json:"orderID"`
	}{OrderID: orderID})
}

func (s *Service) handleOrderUpdate(w http.ResponseWriter, r *http.Request) {
	defer cleanup(r)

	order, err := s.retrieveOrder(r)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusBadRequest, "Bad request")
		return
	}

	if !s.isAllowed(r.Context(), toResource(order), "UPDATE") {
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
}

func (s *Service) handleOrderDelete(w http.ResponseWriter, r *http.Request) {
	defer cleanup(r)

	order, err := s.retrieveOrder(r)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusBadRequest, "Bad request")
		return
	}

	if !s.isAllowed(r.Context(), toResource(order), "DELETE") {
		writeMessage(w, http.StatusForbidden, "Operation not allowed")
		return
	}

	if err := s.orders.Delete(order.ID); err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusInternalServerError, "Failed to delete order")
		return
	}
}

func (s *Service) handleOrderView(w http.ResponseWriter, r *http.Request) {
	defer cleanup(r)

	order, err := s.retrieveOrder(r)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusBadRequest, "Bad request")
		return
	}

	if !s.isAllowed(r.Context(), toResource(order), "VIEW") {
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
		writeMessage(w, http.StatusBadRequest, "Bad request")
		return
	}

	vars := mux.Vars(r)
	status := vars["status"]

	resource := toResource(order).WithAttr("newStatus", status)
	if !s.isAllowed(r.Context(), resource, "UPDATE_STATUS") {
		writeMessage(w, http.StatusForbidden, "Operation not allowed")
		return
	}

	if err := s.orders.SetStatus(order.ID, status); err != nil {
		log.Printf("ERROR: %v", err)
		writeMessage(w, http.StatusInternalServerError, "Failed to update order")
		return
	}
}

func (s *Service) retrieveOrder(r *http.Request) (db.Order, error) {
	vars := mux.Vars(r)

	orderID, err := strconv.ParseUint(vars["orderID"], 10, 64)
	if err != nil {
		return db.Order{}, err
	}

	return s.orders.Get(orderID)
}

func (s *Service) isAllowed(ctx context.Context, resource *cerbos.Resource, action string) bool {
	principal := getPrincipal(ctx)
	if principal == nil {
		return false
	}

	allowed, err := s.cerbos.IsAllowed(ctx, principal, resource, action)
	if err != nil {
		log.Printf("ERROR: %v", err)
		return false
	}

	return allowed
}

func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	defer cleanup(r)

	fmt.Fprintln(w, "OK")
}

type genericResponse struct {
	Message string `json:"message"`
}

func toResource(o db.Order) *cerbos.Resource {
	return cerbos.NewResource("order", strconv.FormatUint(o.ID, 10)).
		WithAttr("items", o.Items).
		WithAttr("status", o.Status).
		WithAttr("owner", o.Owner)
}

func getPrincipal(ctx context.Context) *cerbos.Principal {
	// Get the principal stored in the context by the authentication middleware.
	p := ctx.Value(principalKey)
	if p == nil {
		return nil
	}

	return p.(*cerbos.Principal)
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
