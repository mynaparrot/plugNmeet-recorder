package config

import (
	"reflect"
	"testing"
)

func TestNormalizeNatsUrls(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		want    []string
		wantErr bool
	}{
		{
			name: "native nats url is kept as is",
			in:   []string{"nats://127.0.0.1:4222"},
			want: []string{"nats://127.0.0.1:4222"},
		},
		{
			name: "websocket urls are kept, path preserved",
			in:   []string{"wss://demo.example.com/ws"},
			want: []string{"wss://demo.example.com/ws"},
		},
		{
			name: "http prefix is mapped to ws",
			in:   []string{"http://127.0.0.1:8222"},
			want: []string{"ws://127.0.0.1:8222"},
		},
		{
			name: "https prefix is mapped to wss",
			in:   []string{"https://demo.example.com/ws"},
			want: []string{"wss://demo.example.com/ws"},
		},
		{
			name: "empty entries and duplicates are dropped",
			in:   []string{" nats://127.0.0.1:4222 ", "", "nats://127.0.0.1:4222"},
			want: []string{"nats://127.0.0.1:4222"},
		},
		{
			name:    "empty list is rejected",
			in:      []string{"", " "},
			wantErr: true,
		},
		{
			name:    "unknown scheme is rejected",
			in:      []string{"mqtt://127.0.0.1:1883"},
			wantErr: true,
		},
		{
			name:    "missing scheme is rejected",
			in:      []string{"127.0.0.1:4222"},
			wantErr: true,
		},
		{
			name:    "mixing websocket with native nats is rejected",
			in:      []string{"nats://127.0.0.1:4222", "wss://demo.example.com/ws"},
			wantErr: true,
		},
		{
			name:    "mixing ws with wss is rejected",
			in:      []string{"ws://127.0.0.1:8222", "wss://demo.example.com/ws"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeNatsUrls(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
