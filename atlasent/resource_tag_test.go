package atlasent

import "testing"

type testInvoice struct {
	ID       string `atlasent:"id"`
	Customer string `atlasent:"attr,name=customer_id"`
	Amount   int    `atlasent:"attr"`
	Ignored  string `atlasent:"-"`
	Internal string // no tag; ignored
}

type testResource struct {
	Name string `atlasent:"id"`
	Kind string `atlasent:"type"`
}

func TestResourceFromBasic(t *testing.T) {
	inv := testInvoice{ID: "inv_42", Customer: "cust_7", Amount: 100, Ignored: "skip"}
	r, err := ResourceFrom(inv, "invoice")
	if err != nil {
		t.Fatalf("ResourceFrom: %v", err)
	}
	if r.ID != "inv_42" || r.Type != "invoice" {
		t.Fatalf("wrong id/type: %+v", r)
	}
	if r.Attributes["customer_id"] != "cust_7" {
		t.Fatalf("attr rename lost: %+v", r.Attributes)
	}
	if r.Attributes["Amount"] != 100 {
		t.Fatalf("default-named attr lost: %+v", r.Attributes)
	}
	if _, ok := r.Attributes["Ignored"]; ok {
		t.Fatalf("skipped field leaked: %+v", r.Attributes)
	}
}

func TestResourceFromTypeTag(t *testing.T) {
	r, err := ResourceFrom(testResource{Name: "x", Kind: "widget"}, "")
	if err != nil {
		t.Fatalf("ResourceFrom: %v", err)
	}
	if r.Type != "widget" {
		t.Fatalf("want type=widget, got %q", r.Type)
	}
	if r.ID != "x" {
		t.Fatalf("want id=x, got %q", r.ID)
	}
}

func TestResourceFromNoType(t *testing.T) {
	type x struct {
		ID string `atlasent:"id"`
	}
	if _, err := ResourceFrom(x{ID: "y"}, ""); err == nil {
		t.Fatal("want error when no type and no defaultType")
	}
}

func TestResourceFromPointer(t *testing.T) {
	inv := &testInvoice{ID: "p1"}
	r, err := ResourceFrom(inv, "invoice")
	if err != nil {
		t.Fatalf("ResourceFrom: %v", err)
	}
	if r.ID != "p1" {
		t.Fatalf("want id=p1, got %q", r.ID)
	}
}

func TestResourceFromNil(t *testing.T) {
	var inv *testInvoice
	if _, err := ResourceFrom(inv, "invoice"); err == nil {
		t.Fatal("want error for nil pointer")
	}
}

func TestResourceFromNonStruct(t *testing.T) {
	if _, err := ResourceFrom("string", "invoice"); err == nil {
		t.Fatal("want error for non-struct")
	}
}

func TestResourceFromBadTagRole(t *testing.T) {
	type bad struct {
		X string `atlasent:"bogus"`
	}
	if _, err := ResourceFrom(bad{}, "t"); err == nil {
		t.Fatal("want error for unknown tag role")
	}
}
