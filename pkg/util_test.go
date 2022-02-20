package pkg

import (
	"testing"
	"time"
)

func TestGetHashRateBySize(t *testing.T) {
	type args struct {
		size      int64
		startTime time.Duration
	}
	tests := []struct {
		name string
		args args
		want float64
	}{
		{"1M", args{size: 1000, startTime: time.Second}, 746.2686567164179},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetHashRateBySize(tt.args.size, tt.args.startTime); got != tt.want {
				t.Errorf("GetHashRateBySize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetHumanizeHashRateBySize(t *testing.T) {
	type args struct {
		hashRate float64
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"1M", args{hashRate: 1}, "1.00 MH/S"},
		{"100M", args{hashRate: 100}, "100.00 MH/S"},
		{"1G", args{hashRate: 1000}, "1.00 G/S"},
		{"10G", args{hashRate: 10000}, "10.00 G/S"},
		{"100G", args{hashRate: 100000}, "100.00 G/S"},
		{"1T", args{hashRate: 1000000}, "1.00 T/S"},
		{"100T", args{hashRate: 100000000}, "100.00 T/S"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetHumanizeHashRateBySize(tt.args.hashRate); got != tt.want {
				t.Errorf("GetHumanizeHashRateBySize() = %v, want %v", got, tt.want)
			}
		})
	}
}
