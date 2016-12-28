package db_test

import (
	"testing"

	"github.com/lucapette/deluminator/db"
)

func TestNewLoaders(t *testing.T) {
	dataSources := []string{
		"postgresql://localhost/test",
		"mysql://localhost/test2",
	}

	loaders, err := db.NewLoaders(dataSources)

	if err != nil {
		t.Fatal(err)
	}

	actual := len(loaders)
	expected := len(dataSources)

	if actual != expected {
		t.Fatalf("Expected %d, got %d", expected, actual)
	}
}
