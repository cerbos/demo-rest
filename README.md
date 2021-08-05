Securing a REST API with Cerbos
===============================

This project demonstrates how to secure a REST API using Cerbos policies. It also shows how to run Cerbos as a sidecar.


How it works
------------

HTTP middleware checks the username and password sent with each request against the user database and builds a Cerbos principal object containing roles and attributes.

```go
principal := cerbos.NewPrincipal(username).
    WithRoles(record.Roles...).
    WithAttr("aisles", record.Aisles).
    WithAttr("ipAddress", r.RemoteAddr)
```

Checking access is as simple as making a call to Cerbos PDP.

```go
resource := cerbos.NewResource("inventory", item.ID).WithAttr("aisle", item.Aisle)
allowed, err := cerbos.IsAllowed(ctx, principal, resource, "DELETE")
```


The Store API
-------------

The example application is a simple REST API exposed by a fictional e-commmerce service. Only authenticated users can access the API.

| Endpoint | Description | Rules |
| -------- | ----------- | ------------ |
| `PUT /store/order`            | Create a new order | Only customers can create orders. Each order must contain at least two items. |
| `GET /store/order/{orderID}`  | View the order | Customers can only view their own orders. Store employees can view any order. |
| `POST /store/order/{orderID}` | Update the order | Customers can update their own orders as long as the status is `PENDING` |
| `DELETE /store/order/{orderID}` | Cancel the order | Customers can cancel their own orders as long the status is `PENDING` |
| `POST /backoffice/order/{orderID}/status/{status}` | Update order status | Pickers can change status from `PENDING` to `PICKING` and `PICKING` to `PICKED`. Dispatchers can change status from `PICKED` to `DISPATCHED`. Managers can change the status to anything. |
| `PUT /backoffice/inventory` | Add new item to inventory | Only buyers who are in charge of that category or managers can add new items |
| `GET /backoffice/inventory/{itemID}` | View item | Any employee can view inventory items |
| `POST /backoffice/inventory/{itemID}` | Update item | Buyers who are in charge of that category can update the item provided that the new price is within 10% of the previous price. Managers can update without any restrictions |
| `DELETE /backoffice/inventory/{itemID}` | Remove item | Only buyers who are in charge of that category or managers can remove items |
| `POST /backoffice/inventory/{itemID}/replenish/{quantity}` | Replenish stock | Only stockers and managers can replenish stock |
| `POST /backoffice/inventory/{itemID}/pick/{quantity}` | Pick stock | Only pickers and managers can pick stock |


The Cerbos policies for the service are in the `cerbos/policies` directory.

- `store_roles.yaml`: A derived roles definition which defines `order-owner` derived role to identify when someone is accessing their own order.
- `order_resource.yaml`: A resource policy for the `order` resource encapsulating the rules listed in the table above.
- `inventory_resource.yaml`: A resource policy for the `inventory` resource encapsulating the rules listed in the table above.


Available users are:

| Username | Password | Roles |
| -------- | -------- | ----- |
| adam     | adamsStrongPassword    | customer |
| bella    | bellasStrongPassword   | customer, employee, manager |
| charlie  | charliesStrongPassword | customer, employee, picker |
| diana    | dianasStrongPassword   | customer, employee, dispatcher |
| eve      | evesStrongPassword     | customer
| florence | florencesStrongPassword| customer, employee, buyer (bakery) |
| george   | georgesStrongPassword  | customer, employee, buyer (dairy) |
| harry    | harrysStrongPassword   | customer, employee, stocker |
| jenny    | jennysStrongPassword   | customer, employee, stocker |


Use `docker-compose` to start the demo. Here Cerbos is configured to run as a sidecar to the application and communicate over a Unix domain socket.

```sh
docker-compose up
```

<details>
<summary><b>Examples</b></summary>


**Adam tries to create an order with a single item**

```sh
curl -i -u adam:adamsStrongPassword -XPUT http://localhost:9999/store/order -d '{"items": {"eggs": 12}}'
```
```
{
  "message": "Operation not allowed"
}
```

**Adam has enough items in the order**

```sh
curl -i -u adam:adamsStrongPassword -XPUT http://localhost:9999/store/order -d '{"items": {"eggs": 12, "milk": 1}}'
```
```
{
  "orderID": 1
}
```

**Adam can view his own order**

```sh
curl -i -u adam:adamsStrongPassword -XGET http://localhost:9999/store/order/1
```
```
{
  "id": 1,
  "items": {
    "eggs": 12,
    "milk": 1
  },
  "owner": "adam",
  "status": "PENDING"
}
```

**Eve cannot view Adam's order**

```sh
curl -i -u eve:evesStrongPassword -XGET http://localhost:9999/store/order/1
```
```
{
  "message": "Operation not allowed"
}
```

**Bella can view Adam's order**

```sh
curl -i -u bella:bellasStrongPassword -XGET http://localhost:9999/store/order/1
```
```
{
  "id": 1,
  "items": {
    "eggs": 12,
    "milk": 1
  },
  "owner": "adam",
  "status": "PENDING"
}
```

**Adam can update his pending order**

```sh
curl -i -u adam:adamsStrongPassword -XPOST http://localhost:9999/store/order/1 -d '{"items": {"eggs": 24, "milk": 1, "bread": 1}}'
```
```
{
  "message": "Order updated"
}
```

**Charlie cannot set order status to PICKED because it is not in PICKING status**

```sh
curl -i -u charlie:charliesStrongPassword -XPOST http://localhost:9999/backoffice/order/1/status/PICKED
```
```
{
  "message": "Operation not allowed"
}
```

**Charlie can set order status to PICKING**

```sh
curl -i -u charlie:charliesStrongPassword -XPOST http://localhost:9999/backoffice/order/1/status/PICKING
```
```
{
  "message": "Order status updated"
}
```

**Adam cannot update his order because it is not pending**

```sh
curl -i -u adam:adamsStrongPassword -XPOST http://localhost:9999/store/order/1 -d '{"items": {"eggs": 24, "milk": 1, "bread": 1}}'
```
```
{
  "message": "Operation not allowed"
}
```

**Florence can add an item to the bakery aisle**

```sh
curl -i -u florence:florencesStrongPassword -XPUT http://localhost:9999/backoffice/inventory -d '{"id":"white_bread", "aisle":"bakery", "price":110}'
```
```
{
  "message": "Item added"
}
```

**Florence cannot add an item to the dairy aisle**

```sh
curl -i -u florence:florencesStrongPassword -XPUT http://localhost:9999/backoffice/inventory -d '{"id":"skimmed_milk", "aisle":"dairy", "price":120}'
```
```
{
  "message": "Operation not allowed"
}
```

**Florence can increase the price of an item up to 10%**

```sh
curl -i -u florence:florencesStrongPassword -XPOST http://localhost:9999/backoffice/inventory/white_bread -d '{"id":"white_bread", "aisle":"bakery", "price":120}'
```
```
{
  "message": "Item updated"
}
```

**Florence cannot increase the price of an item more than 10%**

```sh
curl -i -u florence:florencesStrongPassword -XPOST http://localhost:9999/backoffice/inventory/white_bread -d '{"id":"white_bread", "aisle":"bakery", "price":220}'
```
```
{
  "message": "Operation not allowed"
}
```

**Bella can increase the price of an item by any amount**

```sh
curl -i -u bella:bellasStrongPassword -XPOST http://localhost:9999/backoffice/inventory/white_bread -d '{"id":"white_bread", "aisle":"bakery", "price":220}'
```
```
{
  "message": "Item updated"
}
```

**Harry can replenish stock**

```sh
curl -i -u harry:harrysStrongPassword -XPOST http://localhost:9999/backoffice/inventory/white_bread/replenish/10
```
```
{
  "newQuantity": 10
}
```

**Harry cannot pick stock**

```sh
curl -i -u harry:harrysStrongPassword -XPOST http://localhost:9999/backoffice/inventory/white_bread/pick/1
```
```
{
  "message": "Operation not allowed"
}
```

**Charlie can pick stock**

```sh
curl -i -u charlie:charliesStrongPassword -XPOST http://localhost:9999/backoffice/inventory/white_bread/pick/1
```
```
{
  "newQuantity": 9
}
```

**Charlie cannot replenish stock**

```sh
curl -i -u charlie:charliesStrongPassword -XPOST http://localhost:9999/backoffice/inventory/white_bread/replenish/10
```
```
{
  "message": "Operation not allowed"
}
```

**Bella can delete an item from inventory**

```sh
curl -i -u bella:bellasStrongPassword -XDELETE http://localhost:9999/backoffice/inventory/white_bread
```
```
{
  "message": "Item deleted"
}
```
</details>


Get help
--------

- Visit the [Cerbos website](https://cerbos.dev)
- [Join the Cerbos community on Slack](http://go.cerbos.io/slack)
- Email us at help@cerbos.dev
