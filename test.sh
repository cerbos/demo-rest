#!/usr/bin/env bash

GREEN=$'\e[32m'
RED=$'\e[31m'
RESET=$'\e[0m'
HOST="http://localhost:9999"

header() {
    printf "\n%b=== %s%b\n" "$GREEN" "$1" "$RESET" 
}

error() {
    printf "%bERROR: %s%b\n" "$RED" "$1" "$RESET" 
    exit 1
}

check() {
    local TITLE="$1"
    local EXPECTED_CODE="$2"
    local USER="$3"
    shift 3

    header "$TITLE"

    echo "curl -i $@"
    echo ""

    OUT=$(mktemp)
    HTTP_CODE=$(curl --silent --output "$OUT" --write-out "%{http_code}" -u "${USER}:${USER}sStrongPassword" "$@")

    cat "$OUT" && rm "$OUT"

    if [[ "$HTTP_CODE" -ne "$EXPECTED_CODE" ]]; then
        error "Expected $EXPECTED_CODE; Got $HTTP_CODE"
    fi
}

check "Adam tries to create an order with a single item" 403 adam -XPUT "${HOST}/store/order" -d '{"items": {"eggs": 12}}'

check "Adam has enough items in the order" 201 adam -XPUT "${HOST}/store/order" -d '{"items": {"eggs": 12, "milk": 1}}'

check "Adam can view his own order" 200 adam -XGET "${HOST}/store/order/1"  

check "Eve cannot view Adam's order" 403 eve -XGET "${HOST}/store/order/1"  

check "Bella can view Adam's order" 200 bella -XGET "${HOST}/store/order/1"  

check "Adam can update his pending order" 200 adam -XPOST "${HOST}/store/order/1" -d '{"items": {"eggs": 24, "milk": 1, "bread": 1}}'

check "Charlie cannot set order status to PICKED because it is not in PICKING status" 403 charlie -XPOST "${HOST}/backoffice/order/1/status/PICKED" 

check "Charlie can set order status to PICKING" 200 charlie -XPOST "${HOST}/backoffice/order/1/status/PICKING" 

check "Adam cannot update his order because it is not pending" 403 adam -XPOST "${HOST}/store/order/1" -d '{"items": {"eggs": 24, "milk": 1, "bread": 1}}'

check "Florence can add an item to the bakery aisle" 201 florence -XPUT "${HOST}/backoffice/inventory" -d '{"id":"white_bread", "aisle":"bakery", "price":110}'

check "Florence cannot add an item to the dairy aisle" 403 florence -XPUT "${HOST}/backoffice/inventory" -d '{"id":"skimmed_milk", "aisle":"dairy", "price":120}'

check "Florence can increase the price of an item up to 10%" 200 florence -XPOST "${HOST}/backoffice/inventory/white_bread" -d '{"id":"white_bread", "aisle":"bakery", "price":120}'

check "Florence cannot increase the price of an item more than 10%" 403 florence -XPOST "${HOST}/backoffice/inventory/white_bread" -d '{"id":"white_bread", "aisle":"bakery", "price":220}'

check "Bella can increase the price of an item by any amount" 200 bella -XPOST "${HOST}/backoffice/inventory/white_bread" -d '{"id":"white_bread", "aisle":"bakery", "price":220}'

check "Harry can replenish stock" 200 harry -XPOST "${HOST}/backoffice/inventory/white_bread/replenish/10"

check "Harry cannot pick stock" 403 harry -XPOST "${HOST}/backoffice/inventory/white_bread/pick/1"

check "Charlie can pick stock" 200 charlie -XPOST "${HOST}/backoffice/inventory/white_bread/pick/1"

check "Charlie cannot replenish stock" 403 charlie -XPOST "${HOST}/backoffice/inventory/white_bread/replenish/10"

check "Bella can delete an item from inventory" 200 bella -XDELETE "${HOST}/backoffice/inventory/white_bread"

