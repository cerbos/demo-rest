Securing a REST API with Cerbos
===============================

This project demonstrates how to secure a REST API using Cerbos policies. It also shows how to run Cerbos as a sidecar.


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


The users are:

| Username | Password | Roles |
| -------- | -------- | ----- |
| adam     | adamsStrongPassword    | customer |
| bella    | bellasStrongPassword   | customer, employee, manager |
| charlie  | charliesStrongPassword | customer, employee, picker |
| diana    | dianasStrongPassword   | customer, employee, dispatcher |
| eve      | evesStrongPassword     | customer


The Cerbos policies for the service are in the `cerbos/policies` directory. 

- `store_roles.yaml`: A derived roles definition which defines `order-owner` derived role to identify when someone is accessing their own order.
- `order_resource.yaml`: A resource policy for the `order` resource encapsulating the rules listed in the first table above.


Use `docker-compose` to start the demo. Here Cerbos is configured to run as a sidecar to the application and communicate over a Unix domain socket.

```sh
docker-compose up
```

Examples
--------

<details>
<summary><b>Adam tries to create an order with a single item -- which is not allowed by the policy</b></summary>


```sh
curl -i -XPUT localhost:9999/store/order -u adam:adamsStrongPassword -d '{"items": {"eggs": 12}}'
```

```
HTTP/1.1 403 Forbidden
Content-Type: application/json

{
  "message": "Operation not allowed"
}
```

</details>


<details>
<summary><b>Adam now has enough items in his order</b></summary>

```sh
curl -i -XPUT localhost:9999/store/order -u adam:adamsStrongPassword -d '{"items": {"eggs": 12, "milk": 1}}'
```

```
HTTP/1.1 201 Created
Content-Type: application/json

{
  "orderID": 2
}
```

</details>


<details>
<summary><b>Adam can view his own order</b></summary>

```sh
curl -i -XGET localhost:9999/store/order/2 -u adam:adamsStrongPassword
```

```
HTTP/1.1 200 OK
Content-Type: application/json

{
  "id": 2,
  "items": {
    "eggs": 12,
    "milk": 1
  },
  "owner": "adam",
  "status": "PENDING"
}
```

</details>


<details>
<summary><b>Eve cannot view Adam’s order</b></summary>

```sh
curl -i -XGET localhost:9999/store/order/2 -u eve:evesStrongPassword
```

```
HTTP/1.1 403 Forbidden
Content-Type: application/json

{
  "message": "Operation not allowed"
}
```

</details>


<details>
<summary><b>Bella can view Adam’s order because she is an employee</b></summary>

```sh
curl -i -XGET localhost:9999/store/order/2 -u bella:bellasStrongPassword
```

```
HTTP/1.1 200 OK
Content-Type: application/json

{
  "id": 2,
  "items": {
    "eggs": 12,
    "milk": 1
  },
  "owner": "adam",
  "status": "PENDING"
}
```

</details>


<details>
<summary><b>Adam can update his order because it is still PENDING</b></summary>

```sh
curl -i -XPOST localhost:9999/store/order/2 -u adam:adamsStrongPassword -d '{"items": {"eggs": 24, "milk": 1, "bread": 1}}'
```

```
HTTP/1.1 200 OK
```

</details>


<details>
<summary><b>Charlie accidentally tries to set order status to PICKED instead of PICKING</b></summary>

```sh
curl -i -XPOST localhost:9999/backoffice/order/2/status/PICKED -u charlie:charliesStrongPassword
```

```
HTTP/1.1 403 Forbidden
Content-Type: application/json

{
  "message": "Operation not allowed"
}
```

</details>


<details>
<summary><b>Charlie starts picking the order</b></summary>

```sh
curl -i -XPOST localhost:9999/backoffice/order/2/status/PICKING -u charlie:charliesStrongPassword
```

```
HTTP/1.1 200 OK
```

</details>


<details>
<summary><b>Adam can no longer edit his order because the status has changed</b></summary>

```sh
curl -i -XDELETE localhost:9999/store/order/2 -u adam:adamsStrongPassword
```

```
HTTP/1.1 403 Forbidden
Content-Type: application/json

{
  "message": "Operation not allowed"
}
```

</details>


<details>
<summary><b>Diana cannot dispatch the order because the status is still PICKING</b></summary>

```sh
curl -i -XPOST localhost:9999/backoffice/order/2/status/DISPATCHED -u diana:dianasStrongPassword
```

```
HTTP/1.1 403 Forbidden
Content-Type: application/json

{
  "message": "Operation not allowed"
}
```

</details>
