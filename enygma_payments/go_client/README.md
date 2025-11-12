## Instruction on how to run the code

Run the following command on the root of the go_client folder

The transaction sample is set for 6 banks where the sender Id is bank with index 0. This bank is transfering a total of 100 tokens to bank with index 1 and 2. To modifiy the value please changes `tx_value` array in the file `main.go`

```javascript
go run . <qtyBank> <value> <senderId> <sk> <previousV> <previousR> <blockHash>

```

#### Example

```javascript
go run . 6 100 0 35 1000 0 4129789127591820896172587
```
