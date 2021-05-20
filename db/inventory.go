// Copyright 2021 Zenauth Ltd.

package db

import (
	"errors"
	"sync"
)

var (
	ErrAlreadyExists = errors.New("already exists")
	ErrNoStock       = errors.New("no stock")
)

type InventoryItem struct {
	ID    string `json:"id"`
	Price uint64 `json:"price"`
	Aisle string `json:"aisle"`
}

type InventoryRecord struct {
	ID       string `json:"id"`
	Price    uint64 `json:"price"`
	Aisle    string `json:"aisle"`
	Quantity int    `json:"quantity"`
}

type Inventory struct {
	mu    sync.RWMutex
	items map[string]*InventoryRecord
}

func NewInventory() *Inventory {
	return &Inventory{items: make(map[string]*InventoryRecord)}
}

func (i *Inventory) Add(item InventoryItem) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, ok := i.items[item.ID]; ok {
		return ErrAlreadyExists
	}

	i.items[item.ID] = &InventoryRecord{
		ID:    item.ID,
		Aisle: item.Aisle,
		Price: item.Price,
	}

	return nil
}

func (i *Inventory) Update(itm InventoryItem) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	item, ok := i.items[itm.ID]
	if !ok {
		return ErrNotFound
	}

	item.Aisle = itm.Aisle
	item.Price = itm.Price

	return nil
}

func (i *Inventory) UpdateQuantity(id string, quantity int) (int, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	item, ok := i.items[id]
	if !ok {
		return 0, ErrNotFound
	}

	newQty := item.Quantity + quantity
	if newQty < 0 {
		return item.Quantity, ErrNoStock
	}

	item.Quantity = newQty

	return item.Quantity, nil
}

func (i *Inventory) Delete(id string) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, ok := i.items[id]; !ok {
		return ErrNotFound
	}

	delete(i.items, id)

	return nil
}

func (i *Inventory) GetItem(id string) (InventoryRecord, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	item, ok := i.items[id]
	if !ok {
		return InventoryRecord{}, ErrNotFound
	}

	return *item, nil
}
