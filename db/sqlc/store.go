package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store provides all queries to execute db queries and transactions
type Store struct {
	*Queries
	coonPool *pgxpool.Pool
}

func NewStore(coonPool *pgxpool.Pool) *Store {
	return &Store{
		coonPool: coonPool,
		Queries:  New(coonPool),
	}
}

// ExecTx executes a function within a database transaction
func (store *Store) execTx(ctx context.Context, fn func(*Queries) error) error {
	tx, err := store.coonPool.Begin(ctx)

	if err != nil {
		return err
	}

	q := New(tx)

	err = fn(q)

	if err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("tx err %v, rb err: %v", err, rbErr)
		}

		return err
	}

	return tx.Commit(ctx)
}

// TransferTxParams contains the input parameter of the transfer
type TransferTxParams struct {
	FromAccountID int64 `json:"from_account_id"`
	ToAccountID   int64 `json:"to_account_id"`
	Amount        int64 `json:"amount"`
}

type TransferTxResult struct {
	Transfer    Transfers `json:"transfer"`
	FromAccount Accounts  `json:"from_account"`
	ToAccount   Accounts  `json:"to_accounts"`
	FromEntry   Entries   `json:"from_entry"`
	ToEntry     Entries   `json:"to_entry"`
}

var txKey = struct{}{}

// TransferTx performs a money transfer from one account to the other.
// It creates the transfer, add account entries, and update accounts' balance within a database transaction
func (store *Store) TransferTx(ctx context.Context, arg TransferTxParams) (TransferTxResult, error) {
	var result TransferTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		txName := ctx.Value(txKey)

		result.Transfer, err = q.CreateTransfer(ctx, CreateTransferParams{
			FromAccountID: arg.FromAccountID,
			ToAccountID:   arg.ToAccountID,
			Amount:        arg.Amount,
		})

		if err != nil {
			return err
		}

		fmt.Println(txName, "create entry 1")
		result.FromEntry, err = q.CreateEntry(ctx, CreateEntryParams{
			AccountID: arg.FromAccountID,
			Amount:    -arg.Amount,
		})

		if err != nil {
			return err
		}

		fmt.Println(txName, "create entry 2")
		result.ToEntry, err = q.CreateEntry(ctx, CreateEntryParams{
			AccountID: arg.ToAccountID,
			Amount:    arg.Amount,
		})

		if err != nil {
			return err
		}

		//TODO: Update Account Balance
		fmt.Println(txName, "Get Account 1 for update")
		account1, err := q.GetAccountForUpdate(ctx, arg.FromAccountID)

		if err != nil {
			return err
		}

		fmt.Println(txName, "Update Account 1")
		result.FromAccount, err = q.UpdateAccount(ctx, UpdateAccountParams{
			ID:      arg.FromAccountID,
			Balance: account1.Balance - arg.Amount,
		})

		if err != nil {
			return err
		}

		fmt.Println(txName, "Get Account 2 for update")
		account2, err := q.GetAccountForUpdate(ctx, arg.ToAccountID)

		if err != nil {
			return err
		}

		fmt.Println(txName, "Update Account 2")
		result.ToAccount, err = q.UpdateAccount(ctx, UpdateAccountParams{
			ID:      arg.ToAccountID,
			Balance: account2.Balance + arg.Amount,
		})

		if err != nil {
			return err
		}

		return nil
	})

	return result, err
}
