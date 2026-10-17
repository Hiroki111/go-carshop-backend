package oauth

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

const testSecret = "0123456789abcdef0123456789abcdef0123456789abcdef"

func mustClient(t *testing.T, id, secret string, scopes ...string) Client {
	t.Helper()

	client, err := NewClient(id, secret, scopes)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	return client
}

func TestNewClient_Validation(t *testing.T) {
	tests := []struct {
		testName string
		id       string
		secret   string
		scopes   []string
	}{
		{testName: "empty ID", id: "", secret: testSecret, scopes: []string{ScopeCarsReserve}},
		{testName: "short secret", id: "order-service", secret: strings.Repeat("a", MinSecretLength-1), scopes: []string{ScopeCarsReserve}},
		{testName: "empty secret", id: "order-service", secret: "", scopes: []string{ScopeCarsReserve}},
		{testName: "no scopes", id: "order-service", secret: testSecret, scopes: nil},
		{testName: "empty scope", id: "order-service", secret: testSecret, scopes: []string{""}},
		{testName: "scope with a space", id: "order-service", secret: testSecret, scopes: []string{"cars:reserve cars:release"}},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			if _, err := NewClient(test.id, test.secret, test.scopes); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

func TestNewClient_DoesNotKeepTheCallersScopeSlice(t *testing.T) {
	scopes := []string{ScopeCarsReserve}
	client := mustClient(t, "order-service", testSecret, scopes...)

	scopes[0] = "something:else"
	if got := client.AllowedScopes(); !reflect.DeepEqual(got, []string{ScopeCarsReserve}) {
		t.Fatalf("expected the client to keep its own copy, got %v", got)
	}

	got := client.AllowedScopes()
	got[0] = "something:else"
	if again := client.AllowedScopes(); !reflect.DeepEqual(again, []string{ScopeCarsReserve}) {
		t.Fatalf("expected AllowedScopes to return a copy, got %v", again)
	}
}

func TestRegistry_Authenticate(t *testing.T) {
	registry, err := NewRegistry(
		mustClient(t, "order-service", testSecret, ScopeCarsReserve),
		mustClient(t, "other-service", strings.Repeat("z", 40), ScopeCarsSold),
	)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		testName string
		id       string
		secret   string
		wantID   string // empty means "must fail"
	}{
		{testName: "right ID and secret", id: "order-service", secret: testSecret, wantID: "order-service"},
		{testName: "another client", id: "other-service", secret: strings.Repeat("z", 40), wantID: "other-service"},
		{testName: "wrong secret", id: "order-service", secret: testSecret + "x"},
		{testName: "another client's secret", id: "order-service", secret: strings.Repeat("z", 40)},
		{testName: "empty secret", id: "order-service", secret: ""},
		{testName: "unknown client", id: "nobody", secret: testSecret},
		{testName: "empty ID", id: "", secret: testSecret},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			client, err := registry.Authenticate(test.id, test.secret)

			if test.wantID == "" {
				if !errors.Is(err, ErrInvalidClient) {
					t.Fatalf("expected ErrInvalidClient, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if client.ID != test.wantID {
				t.Fatalf("expected client %q, got %q", test.wantID, client.ID)
			}
		})
	}
}

func TestRegistry_NilRegistryAuthenticatesNobody(t *testing.T) {
	var registry *Registry

	if _, err := registry.Authenticate("order-service", testSecret); !errors.Is(err, ErrInvalidClient) {
		t.Fatalf("expected ErrInvalidClient, got %v", err)
	}
}

func TestNewRegistry_RejectsDuplicateClientIDs(t *testing.T) {
	_, err := NewRegistry(
		mustClient(t, "order-service", testSecret, ScopeCarsReserve),
		mustClient(t, "order-service", testSecret, ScopeCarsSold),
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestClient_ResolveScopes(t *testing.T) {
	client := mustClient(t, "order-service", testSecret, ScopeCarsReserve, ScopeCarsRelease, ScopeCarsSold)

	tests := []struct {
		testName  string
		requested string
		want      []string
		wantErr   error
	}{
		{testName: "nothing requested means everything allowed", requested: "", want: []string{ScopeCarsReserve, ScopeCarsRelease, ScopeCarsSold}},
		{testName: "only whitespace means everything allowed", requested: "  ", want: []string{ScopeCarsReserve, ScopeCarsRelease, ScopeCarsSold}},
		{testName: "one scope", requested: "cars:release", want: []string{ScopeCarsRelease}},
		{testName: "two scopes keep the requested order", requested: "cars:sold cars:reserve", want: []string{ScopeCarsSold, ScopeCarsReserve}},
		{testName: "repeated scope is granted once", requested: "cars:reserve cars:reserve", want: []string{ScopeCarsReserve}},
		{testName: "extra whitespace", requested: "  cars:reserve   cars:sold ", want: []string{ScopeCarsReserve, ScopeCarsSold}},

		{testName: "unknown scope", requested: "cars:delete", wantErr: ErrInvalidScope},
		{testName: "look-alike scope", requested: "cars:reserved", wantErr: ErrInvalidScope},
		// All or nothing: one bad scope doesn't get the rest granted.
		{testName: "allowed and unknown scope", requested: "cars:reserve cars:delete", wantErr: ErrInvalidScope},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			got, err := client.ResolveScopes(test.requested)

			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("expected %v, got %v", test.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("expected %v, got %v", test.want, got)
			}
		})
	}
}
