package dbutils

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func insertUser(ctx context.Context, txn *sql.Tx, req *struct{}) error {
	_, err := txn.ExecContext(ctx, "INSERT INTO users (name, age) VALUES (?, ?)", "Alice", 25)
	return err
}

func updateInventory(ctx context.Context, txn *sql.Tx, req *struct{}) error {
	_, err := txn.ExecContext(ctx, "UPDATE inventory SET stock = stock - ? WHERE product_id = ?", 10, 1)
	return err
}

func TestSqlWriteExec_Success(t *testing.T) {
	// Mock database and transaction
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// Mock BeginTx
	mock.ExpectBegin()

	// Mock SQL queries
	mock.ExpectExec("INSERT INTO users").WithArgs("Alice", 25).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE inventory").WithArgs(10, 1).WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock Commit
	mock.ExpectCommit()

	// Execute
	err = NewSqlTxnExec[struct{}, any](ctx, db, nil, nil).
		Exec(insertUser).
		Exec(updateInventory).
		Commit()

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSqlWriteExec_Failure(t *testing.T) {
	// Mock database and transaction
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// Mock BeginTx
	mock.ExpectBegin()

	// Mock SQL query that fails
	mock.ExpectExec("INSERT INTO users").WithArgs("Alice", 25).WillReturnError(errors.New("insert failed"))

	// Mock Rollback
	mock.ExpectRollback()

	// Execute
	err = NewSqlTxnExec[struct{}, struct{}](ctx, db, nil, nil).
		Exec(insertUser).
		Commit()

	assert.Error(t, err)
	assert.EqualError(t, err, "insert failed")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSqlWriteExec_EmptyTransaction(t *testing.T) {
	// Mock database and transaction
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// Mock BeginTx
	mock.ExpectBegin()

	// Mock Commit for an empty transaction
	mock.ExpectCommit()

	// Execute
	exec := NewSqlTxnExec[any, any](ctx, db, nil, nil)

	err = exec.Commit()
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Structs for test inputs and results
type OrderRequest struct {
	CustomerName string
	Payment      struct {
		Method     string
		CreditCard struct {
			CardNumber string
		}
	}
	TotalAmount float64
}

type ProcessedResponse struct {
	OrderID int64
}

// Mocked stateful writer functions
func insertOrder(ctx context.Context, txn *sql.Tx, orderReq *OrderRequest, processedRes *ProcessedResponse) error {
	result, err := txn.ExecContext(ctx, "INSERT INTO orders (customer_name, total_amount) VALUES (?, ?)",
		orderReq.CustomerName,
		orderReq.TotalAmount,
	)
	if err != nil {
		return err
	}
	orderID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	processedRes.OrderID = orderID
	return nil
}

func insertPayment(ctx context.Context, txn *sql.Tx, orderReq *OrderRequest, processedRes *ProcessedResponse) error {

	_, err := txn.ExecContext(ctx, "INSERT INTO payments (order_id, amount, payment_method) VALUES (?, ?, ?)",
		processedRes.OrderID,
		orderReq.TotalAmount,
		orderReq.Payment.Method,
	)
	return err
}

func TestSqlWriteExec_StatefulOperations(t *testing.T) {
	// Initialize mock database and context
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// Define test data
	orderReq := &OrderRequest{
		CustomerName: "John Doe",
		TotalAmount:  100.50,
		Payment: struct {
			Method     string
			CreditCard struct{ CardNumber string }
		}{
			Method: "Credit Card",
			CreditCard: struct{ CardNumber string }{
				CardNumber: "1234-5678-9012-3456",
			},
		},
	}

	// Mock SQL expectations
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO orders").
		WithArgs("John Doe", 100.50).
		WillReturnResult(sqlmock.NewResult(1, 1)) // Simulating Order ID = 1
	mock.ExpectExec("INSERT INTO payments").
		WithArgs(1, 100.50, "Credit Card").
		WillReturnResult(sqlmock.NewResult(2, 1)) // Simulating Payment ID = 2
	mock.ExpectCommit()

	// Initialize SqlWriteExec
	err = NewSqlTxnExec[OrderRequest, ProcessedResponse](ctx, db, nil, orderReq).
		StatefulExec(insertOrder).
		StatefulExec(insertPayment).
		Commit()

	assert.NoError(t, err)

	// Verify mock expectations
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Stateful writer function that succeeds and updates the response.
func successfulStatefulInsert(ctx context.Context, txn *sql.Tx, orderReq *OrderRequest, processedRes *ProcessedResponse) error {
	res, err := txn.ExecContext(ctx, "INSERT INTO orders (customer_name, total_amount) VALUES (?, ?)",
		orderReq.CustomerName,
		orderReq.TotalAmount,
	)
	if err != nil {
		return err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return err
	}

	processedRes.OrderID = id
	return nil
}

// Stateful writer function that fails.
func failingStatefulInsert(ctx context.Context, txn *sql.Tx, orderReq *OrderRequest, processedRes *ProcessedResponse) error {
	return errors.New("forced error during stateful insert")
}

func TestSqlWriteExec_StatefulRollbackOnFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// Define test data
	orderReq := &OrderRequest{
		CustomerName: "Jane Doe",
		TotalAmount:  300.50,
	}

	processedRes := &ProcessedResponse{}

	// Mock SQL expectations
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO orders").
		WithArgs("Jane Doe", 300.50).
		WillReturnResult(sqlmock.NewResult(1, 1)) // Simulating successful insert
	mock.ExpectRollback() // Expect a rollback due to the forced error in the second operation

	// Initialize SqlWriteExec
	err = NewSqlTxnExec[OrderRequest, ProcessedResponse](ctx, db, nil, orderReq).
		StatefulExec(successfulStatefulInsert).
		StatefulExec(failingStatefulInsert).
		Commit()

	assert.Error(t, err)
	assert.EqualError(t, err, "forced error during stateful insert")

	// Verify that the response wasn't updated due to rollback
	assert.Equal(t, int64(0), processedRes.OrderID)

	// Verify mock expectations
	assert.NoError(t, mock.ExpectationsWereMet())
}
