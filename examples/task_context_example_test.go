package examples

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mahadev-k/go-utils/goctx"
	streams "github.com/mahadev-k/go-utils/stream_utils"
)

// Domain types
type OrderItem struct {
	ProductID string
	Quantity  int
}

type Shipment struct {
	OrderID     string
	Status      string
	TrackingNum string
}

// Inventory operations
func checkInventory(item OrderItem) goctx.RunFn[bool] {
	return func() (bool, error) {
		// Simulate DB check
		time.Sleep(100 * time.Millisecond)
		if item.ProductID == "LAPTOP" && item.Quantity > 1 {
			return false, fmt.Errorf("insufficient inventory for %s", item.ProductID)
		}
		return true, nil
	}
}

// Shipping operations
func validateAddress(address string) goctx.RunFn[bool] {
	return func() (bool, error) {
		time.Sleep(50 * time.Millisecond)
		if address == "" {
			return false, errors.New("invalid address")
		}
		return true, nil
	}
}

func calculateShipping(items []OrderItem) goctx.RunFn[float64] {
	return func() (float64, error) {
		time.Sleep(100 * time.Millisecond)
		var total float64
		for _, item := range items {
			total += float64(item.Quantity) * 5.99
		}
		return total, nil
	}
}

func generateLabel(orderID string, cost float64) goctx.RunFn[string] {
	return func() (string, error) {
		if cost <= 0 {
			return "", errors.New("invalid shipping cost")
		}
		return fmt.Sprintf("TRACK-%s-1234567890", orderID), nil
	}
}

func ExampleTaskContext_OrderProcessing() {
	ctx := goctx.NewTaskContext(context.Background())

	// Mock order
	order := []OrderItem{
		{ProductID: "LAPTOP", Quantity: 2},
		{ProductID: "MOUSE", Quantity: 3},
	}

	taskCtx := goctx.NewTaskContext(ctx)

	// Create inventory checks for each item
	inventoryChecks := goctx.Run[[]goctx.RunFn[bool]](taskCtx,
		func() ([]goctx.RunFn[bool], error) {
			return streams.NewTransformer[OrderItem, goctx.RunFn[bool]](order).
				Transform(streams.MapItSimple(checkInventory)).
				Result()
		})

	// Run inventory checks in parallel
	_, err := goctx.RunParallel(ctx, inventoryChecks...)
	fmt.Printf("Inventory check error: %v\n", err)

	// Output:
	// Inventory check error: task 1: insufficient inventory for LAPTOP
}

func ExampleTaskContext_ShipmentProcessing() {
	ctx := goctx.NewTaskContext(context.Background())

	order := dummyOrder()
	shipment := dummyShipment()

	// Step 1: Validate address
	// Step 2: Calculate shipping cost
	// Step 3: Generate label
	_ = goctx.Run(ctx, validateAddress("123 Main St"))
	cost := goctx.Run(ctx, calculateShipping(order))
	trackingNum := goctx.Run(ctx, generateLabel(shipment.OrderID, cost))

	if ctx.Err() != nil {
		fmt.Printf("Error: %v\n", ctx.Err())
		return
	}

	shipment.Status = "READY"
	shipment.TrackingNum = trackingNum
	fmt.Printf("Shipment processed: %+v\n", shipment)

	// Output:
	// Shipment processed: {OrderID:ORD123 Status:READY TrackingNum:TRACK-ORD123-1234567890}
}

func ExampleTaskContext_ShipmentProcessing_WithError() {
	ctx := goctx.NewTaskContext(context.Background())

	order := dummyOrder()
	shipment := dummyShipment()

	goctx.Run(ctx, validateAddress(""))
	cost := goctx.Run(ctx, calculateShipping(order))
	goctx.Run(ctx, generateLabel(shipment.OrderID, cost))

	if ctx.Err() != nil {
		fmt.Printf("Error: %v\n", ctx.Err())
		return
	}

	fmt.Printf("Shipment processed: %+v\n", shipment)

	// Output:
	// Error: invalid address
}

func dummyOrder() []OrderItem {
	return []OrderItem{
		{ProductID: "MOUSE", Quantity: 1},
	}
}

func dummyShipment() Shipment {
	return Shipment{OrderID: "ORD123"}
}
