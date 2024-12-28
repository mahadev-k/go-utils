package examples

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/mahadev-k/go-utils/dbutils"
	"github.com/stretchr/testify/assert"
	_ "modernc.org/sqlite" // Import pure Go sqlite driver
)

type OrderRequest struct {
	CustomerName string
	ProductID    int
	Quantity     int
}

type OrderProcessingResponse struct {
	OrderID    int64
	ShippingID int64
}

func TestSqlWriteExec_CreateOrderTxn(t *testing.T) {

	db := setupDatabase()
	// create a new SQL Write Executor
	err := dbutils.NewSqlTxnExec[OrderRequest, OrderProcessingResponse](context.TODO(), db, nil, &OrderRequest{CustomerName: "CustomerA", ProductID: 1, Quantity: 10}).
		StatefulExec(InsertOrder).
		StatefulExec(UpdateInventory).
		StatefulExec(InsertShipment).
		Commit()
	// check if the transaction was committed successfully
	if err != nil {
		t.Fatal(err)
		return
	}
	verifyTransactionSuccessful(t, db)
	t.Cleanup(
		func() {
			cleanup(db)
			db.Close()
		},
	)
}

func TestRollback(t *testing.T) {
	db := setupDatabase()

	err := dbutils.NewSqlTxnExec[OrderRequest, OrderProcessingResponse](context.TODO(), db, nil, &OrderRequest{CustomerName: "CustomerA", ProductID: 1, Quantity: 30}).
		StatefulExec(InsertOrder).
		StatefulExec(UpdateInventory).
		StatefulExec(InsertShipment).
		Commit()

	// check if the transaction was rolled back successfully
	if err == nil {
		t.Fatal("Expected error during rollback, but none occurred")
		return
	}

	verifyTransactionFailed(t, db)
	t.Cleanup(
		func() {
			cleanup(db)
			db.Close()
		},
	)
}

func verifyTransactionFailed(t *testing.T, db *sql.DB) {
	// check if the transaction was rolled back successfully
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&count)
	if err != nil {
		t.Fatal(err)
		return
	}
	if count != 0 {
		t.Errorf("Expected 0 orders, but got %d", count)
		return
	}

}

func verifyTransactionSuccessful(t *testing.T, db *sql.DB) {
	// Verify the transaction data
	// Check if the Order, Inventory, and Shipment were inserted and updated correctly
	// run select queries against the database to verify the results
	var orderID int64
	var shippingID int64
	var productQuantity int

	row := db.QueryRow("SELECT id FROM orders WHERE customer_name =?", "CustomerA")
	err := row.Scan(&orderID)
	if err != nil {
		t.Fatal(err)
		return
	}

	row = db.QueryRow("SELECT product_quantity FROM inventory WHERE id =?", 1)
	err = row.Scan(&productQuantity)
	if err != nil {
		t.Fatal(err)
		return
	}
	t.Log("Order ID: ", orderID)
	t.Log("Product Quantity: ", productQuantity)
	t.Log("Shipping ID: ", shippingID)

	assert.Equal(t, productQuantity, 10)
}

func InsertOrder(ctx context.Context, txn *sql.Tx, order *OrderRequest, orderProcessing *OrderProcessingResponse) error {
	// Insert Order
	result, err := txn.Exec("INSERT INTO orders (customer_name, product_id, quantity) VALUES ($1, $2, $3)", order.CustomerName, order.ProductID, order.Quantity)
	if err != nil {
		return err
	}
	// Get the inserted Order ID
	orderProcessing.OrderID, err = result.LastInsertId()
	return err
}

func UpdateInventory(ctx context.Context, txn *sql.Tx, order *OrderRequest, orderProcessing *OrderProcessingResponse) error {
	// Update Inventory if it exists and the quantity is greater than the quantity check if it exists
	result, err := txn.Exec("UPDATE inventory SET product_quantity = product_quantity - $1 WHERE id = $2 AND product_quantity >= $1", order.Quantity, order.ProductID)
	if err != nil {
		return err
	}
	// Get the number of rows affected
	rowsAffected, err := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.New("Insufficient inventory")
	}
	return err
}

func InsertShipment(ctx context.Context, txn *sql.Tx, order *OrderRequest, orderProcessing *OrderProcessingResponse) error {
	// Insert Shipment
	result, err := txn.Exec("INSERT INTO shipping_info (customer_name, shipping_address) VALUES ($1, 'Shipping Address')", order.CustomerName)
	if err != nil {
		return err
	}
	// Get the inserted Shipping ID
	orderProcessing.ShippingID, err = result.LastInsertId()
	return err
}

func setupDatabase() *sql.DB {
	// Arrange
	// create a sqllite db
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}

	// use the database to execute the query
	query := `CREATE TABLE IF NOT EXISTS orders (id INTEGER PRIMARY KEY, 
				customer_name TEXT NOT NULL,
				product_id INTEGER NOT NULL,
				quantity INTEGER NOT NULL);`
	_, err = db.Exec(query)
	if err != nil {
		panic(err)
	}

	//create inventory
	query = `CREATE TABLE IF NOT EXISTS inventory (id INTEGER PRIMARY KEY, product_name TEXT NOT NULL
	, product_quantity INTEGER NOT NULL);`
	_, err = db.Exec(query)
	if err != nil {
		panic(err)
	}

	//create shipping information
	query = `CREATE TABLE IF NOT EXISTS shipping_info (id INTEGER PRIMARY KEY, customer_name TEXT NOT
	NULL, shipping_address TEXT NOT NULL);`
	_, err = db.Exec(query)
	if err != nil {
		panic(err)
	}

	// insert some data
	query = `INSERT INTO inventory (id, product_name, product_quantity) VALUES (1, "Product1", 20);`
	_, err = db.Exec(query)
	if err != nil {
		panic(err)
	}

	return db
}

func cleanup(db *sql.DB) error {
	_, err := db.Exec("DROP TABLE IF EXISTS Order")
	if err != nil {
		return err
	}
	_, err = db.Exec("DROP TABLE IF EXISTS inventory")
	if err != nil {
		return err
	}
	_, err = db.Exec("DROP TABLE IF EXISTS shipment")
	if err != nil {
		return err
	}
	return nil
}
