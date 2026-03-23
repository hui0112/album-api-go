package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL driver — the underscore means
	// "import for side effects only." The driver registers itself with
	// database/sql so we can use sql.Open("mysql", ...).
)

// ============================================================================
// MySQLStore — implements the CartStore interface using MySQL
// ============================================================================

type MySQLStore struct {
	db *sql.DB // This IS the connection pool. sql.DB manages multiple
	// connections internally — we don't need to manage them ourselves.
}

// NewMySQLStore creates a new MySQL connection pool and initializes the schema.
func NewMySQLStore() (*MySQLStore, error) {
	// Build the connection string (DSN = Data Source Name)
	// Format: user:password@tcp(host:port)/database?options
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		os.Getenv("RDS_USER"),
		os.Getenv("RDS_PASSWORD"),
		os.Getenv("RDS_HOST"),
		os.Getenv("RDS_PORT"),
		os.Getenv("RDS_DATABASE"),
	)

	// sql.Open doesn't actually connect — it just validates the DSN(Data source Name)
	// and sets up the connection pool configuration.
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Connection pool settings
	db.SetMaxOpenConns(25)                 // Max simultaneous connections to MySQL
	db.SetMaxIdleConns(10)                 // Keep 10 connections ready in the pool
	db.SetConnMaxLifetime(5 * time.Minute) // Recycle connections after 5 min

	// NOW actually connect and verify — Ping sends a real request to MySQL.
	// We retry because RDS might still be starting up.
	for i := 0; i < 30; i++ {
		err = db.Ping()
		if err == nil {
			break
		}
		log.Printf("Waiting for MySQL... attempt %d/30: %v", i+1, err)
		time.Sleep(2 * time.Second) // maximum 60 seconds waiting
	}
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL after retries: %w", err)
	}
	log.Println("Connected to MySQL successfully")

	store := &MySQLStore{db: db}

	// Auto-create tables on startup (idempotent — IF NOT EXISTS)
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates the tables if they don't exist. Auto migrate.
// This runs every time the app starts, but IF NOT EXISTS makes it safe.
func (s *MySQLStore) initSchema() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Table 1: carts — one row per shopping cart
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS carts (
			id          INT AUTO_INCREMENT PRIMARY KEY,
			customer_id INT NOT NULL,
			created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_customer_id (customer_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create carts table: %w", err)
	}

	// Table 2: cart_items — one row per item in a cart
	// UNIQUE(cart_id, product_id) prevents duplicate products in the same cart
	// and enables INSERT ... ON DUPLICATE KEY UPDATE.
	_, err = s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS cart_items (
			id          INT AUTO_INCREMENT PRIMARY KEY,
			cart_id     INT NOT NULL,
			product_id  INT NOT NULL,
			quantity    INT NOT NULL DEFAULT 1,
			created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (cart_id) REFERENCES carts(id) ON DELETE CASCADE,
			UNIQUE INDEX idx_cart_product (cart_id, product_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create cart_items table: %w", err)
	}

	log.Println("MySQL schema initialized")
	return nil
}

// ============================================================================
// CreateCart — INSERT a new cart, return the auto-generated ID
// ============================================================================
func (s *MySQLStore) CreateCart(customerID int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// ExecContext for INSERT/UPDATE/DELETE (no rows returned).
	// The ? placeholder prevents SQL injection — the driver safely escapes
	// the customerID value before sending it to MySQL.
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO carts (customer_id) VALUES (?)", customerID)
	if err != nil {
		return 0, err
	}

	// LastInsertId returns the AUTO_INCREMENT value MySQL generated.
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

// ============================================================================
// GetCart — SELECT cart + items using LEFT JOIN
// ============================================================================
// LEFT JOIN means: "return the cart even if it has zero items."
// An INNER JOIN would return nothing for empty carts.
func (s *MySQLStore) GetCart(cartID int) (*Cart, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.customer_id, c.created_at,
		       ci.product_id, ci.quantity
		FROM carts c
		LEFT JOIN cart_items ci ON c.id = ci.cart_id
		WHERE c.id = ?
	`, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() // ALWAYS close rows to return the connection to the pool

	var cart *Cart
	for rows.Next() {
		var (
			id         int
			customerID int
			createdAt  time.Time
			productID  sql.NullInt64 // NullInt64 because LEFT JOIN can produce NULLs
			quantity   sql.NullInt64 // (when the cart has no items)
		)

		if err := rows.Scan(&id, &customerID, &createdAt, &productID, &quantity); err != nil {
			return nil, err
		}

		// First row — initialize the cart
		if cart == nil {
			cart = &Cart{
				ID:         id,
				CustomerID: customerID,
				Items:      []CartItem{},
				CreatedAt:  createdAt.Format(time.RFC3339),
			}
		}

		// If this row has an item (not NULL from LEFT JOIN), add it
		if productID.Valid {
			cart.Items = append(cart.Items, CartItem{
				ProductID: int(productID.Int64),
				Quantity:  int(quantity.Int64),
			})
		}
	}

	return cart, rows.Err()
}

// ============================================================================
// AddItem — INSERT or UPDATE item in a cart (using transaction)
// ============================================================================
// Uses INSERT ... ON DUPLICATE KEY UPDATE:
//   - If the (cart_id, product_id) pair doesn't exist → INSERT
//   - If it already exists (UNIQUE constraint hit) → UPDATE the quantity
// This is wrapped in a transaction with the cart's updated_at update.
func (s *MySQLStore) AddItem(cartID int, productID int, quantity int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start a transaction — both operations must succeed or both fail
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // No-op if Commit() succeeds

	// 1, Upsert the item (insert or update items)
	// (Parameterized Query)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO cart_items (cart_id, product_id, quantity)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE quantity = VALUES(quantity)
	`, cartID, productID, quantity)
	if err != nil {
		return err
	}

	// 2. Update the cart's timestamp
	_, err = tx.ExecContext(ctx,
		"UPDATE carts SET updated_at = NOW() WHERE id = ?", cartID)
	if err != nil {
		return err
	}

	// commit transaction only if both 1 and 2 succeded
	return tx.Commit()
}

// Close shuts down the connection pool gracefully.
func (s *MySQLStore) Close() error {
	log.Println("Closing MySQL connection pool")
	return s.db.Close()
}
