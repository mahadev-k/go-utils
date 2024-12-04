package dbutils

import (
	"context"
	"database/sql"
	"errors"
)

type TxnFn[T any] func(ctx context.Context, txn *sql.Tx, processingReq *T) error
type StatefulTxnFn[T any, R any] func(ctx context.Context, txn *sql.Tx, processingReq *T, processedRes *R) error

// SQL Write Executor is responsible when executing write operations
// For dependent writes you may need to add the dependent data to processReq and proceed to the next function call
type SqlTxnExec[T any, R any] struct {
	db               *sql.DB
	txn              *sql.Tx
	txnFns         []TxnFn[T]
	statefulTxnFns []StatefulTxnFn[T, R]
	processingReq    *T
	processedRes     *R
	ctx              context.Context
	err              error
}

func NewSqlTxnExec[T any, R any](ctx context.Context, db *sql.DB, opts *sql.TxOptions, processingReq *T) *SqlTxnExec[T, R] {
	tx, err := db.BeginTx(ctx, opts)
	var processedRes R
	return &SqlTxnExec[T, R]{
		ctx:           ctx,
		db:            db,
		txn:           tx,
		processingReq: processingReq,
		processedRes:  &processedRes,
		err:           err,
	}
}

func (s *SqlTxnExec[T, R]) Exec(txnFn TxnFn[T]) *SqlTxnExec[T, R] {
	s.txnFns = append(s.txnFns, txnFn)
	return s
}

func (s *SqlTxnExec[T, R]) StatefulExec(statefulTxnFn StatefulTxnFn[T, R]) *SqlTxnExec[T, R] {
	s.statefulTxnFns = append(s.statefulTxnFns, statefulTxnFn)
	return s
}

func (s *SqlTxnExec[T, R]) Commit() (err error) {
	defer func() {
		if p := recover(); p != nil {
			s.txn.Rollback()
			panic(p)
		} else if err != nil {
			err = errors.Join(err, s.txn.Rollback())
		} else {
			err = errors.Join(err, s.txn.Commit())
		}
		return
	}()

	for _, writeFn := range s.txnFns {
		if err = writeFn(s.ctx, s.txn, s.processingReq); err != nil {
			return
		}
	}

	for _, statefulWriteFn := range s.statefulTxnFns {
		if err = statefulWriteFn(s.ctx, s.txn, s.processingReq, s.processedRes); err != nil {
			return
		}
	}
	return
}
