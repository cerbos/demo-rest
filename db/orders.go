package db

import (
	"errors"
	"sync"
)

var ErrNotFound = errors.New("not found")

type CustomerOrder struct {
	Items map[string]uint `json:"items"`
}

type Order struct {
	ID     uint64          `json:"id"`
	Items  map[string]uint `json:"items"`
	Owner  string          `json:"owner"`
	Status string          `json:"status"`
}

type OrderDB struct {
	mu           sync.RWMutex
	orderCounter uint64
	orders       map[uint64]*Order
}

func NewOrderDB() *OrderDB {
	return &OrderDB{
		orders: make(map[uint64]*Order),
	}
}

func (odb *OrderDB) Create(owner string, order CustomerOrder) uint64 {
	odb.mu.Lock()
	defer odb.mu.Unlock()

	odb.orderCounter++
	odb.orders[odb.orderCounter] = &Order{
		ID:     odb.orderCounter,
		Items:  order.Items,
		Owner:  owner,
		Status: "PENDING",
	}

	return odb.orderCounter
}

func (odb *OrderDB) Update(orderID uint64, order CustomerOrder) error {
	odb.mu.Lock()
	defer odb.mu.Unlock()

	o, ok := odb.orders[orderID]
	if !ok {
		return ErrNotFound
	}

	o.Items = order.Items

	return nil
}

func (odb *OrderDB) Delete(orderID uint64) error {
	odb.mu.Lock()
	defer odb.mu.Unlock()

	_, ok := odb.orders[orderID]
	if !ok {
		return ErrNotFound
	}

	delete(odb.orders, orderID)

	return nil
}

func (odb *OrderDB) Get(orderID uint64) (Order, error) {
	odb.mu.RLock()
	defer odb.mu.RUnlock()

	o, ok := odb.orders[orderID]
	if !ok {
		return Order{}, ErrNotFound
	}

	return *o, nil
}

func (odb *OrderDB) SetStatus(orderID uint64, status string) error {
	odb.mu.Lock()
	defer odb.mu.Unlock()

	o, ok := odb.orders[orderID]
	if !ok {
		return ErrNotFound
	}

	o.Status = status

	return nil
}
