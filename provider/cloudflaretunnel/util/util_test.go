package util

import (
	"reflect"
	"testing"

	cloudflare "github.com/cloudflare/cloudflare-go"
)

func TestOrderedMap_Len(t *testing.T) {
	type args struct {
		s []cloudflare.UnvalidatedIngressRule
	}
	tests := []struct {
		name string
		n    *OrderedMap
		args args
		want int
	}{
		{
			name: "Empty",
			n:    NewOrderedMap(0),
			args: args{
				s: []cloudflare.UnvalidatedIngressRule{},
			},
			want: 0,
		},
		{
			name: "No data",
			n:    NewOrderedMap(3),
			args: args{
				s: []cloudflare.UnvalidatedIngressRule{},
			},
			want: 0,
		},
		{
			name: "With data",
			n:    NewOrderedMap(3),
			args: args{
				s: []cloudflare.UnvalidatedIngressRule{
					{
						Hostname: "test1",
					},
					{
						Hostname: "test2",
					},
					{
						Hostname: "test3",
					},
				},
			},
			want: 3,
		},
	}
	for _, tt := range tests {
		for _, s := range tt.args.s {
			tt.n.Add(s)
		}
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.n.Len(); got != tt.want {
				t.Errorf("OrderedMap.Len() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrderedMap_Add(t *testing.T) {
	type args struct {
		s []cloudflare.UnvalidatedIngressRule
	}
	tests := []struct {
		name string
		n    *OrderedMap
		args args
		want []cloudflare.UnvalidatedIngressRule
	}{
		{
			name: "Add without duplicate",
			n:    NewOrderedMap(3),
			args: args{
				s: []cloudflare.UnvalidatedIngressRule{
					{
						Hostname: "test1",
					},
					{
						Hostname: "test2",
					},
					{
						Hostname: "test3",
					},
				},
			},
			want: []cloudflare.UnvalidatedIngressRule{
				{
					Hostname: "test1",
				},
				{
					Hostname: "test2",
				},
				{
					Hostname: "test3",
				},
			},
		},
		{
			name: "Add with duplicate",
			n:    NewOrderedMap(10),
			args: args{
				s: []cloudflare.UnvalidatedIngressRule{
					{
						Hostname: "test1",
					},
					{
						Hostname: "test2",
					},
					{
						Hostname: "test3",
					},
					{
						Hostname: "test1",
					},
					{
						Hostname: "test2",
					},
					{
						Hostname: "test1",
					},
				},
			},
			want: []cloudflare.UnvalidatedIngressRule{
				{
					Hostname: "test3",
				},
				{
					Hostname: "test2",
				},
				{
					Hostname: "test1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, s := range tt.args.s {
				tt.n.Add(s)
			}
			values := tt.n.Get()
			if len(values) != len(tt.want) {
				t.Errorf("Expected %d, got %d", len(tt.want), len(values))
			}
			if !reflect.DeepEqual(values, tt.want) {
				t.Errorf("Expected %v, got %v", tt.want, values)
			}
		})
	}
}

func TestOrderedMap_Update(t *testing.T) {
	type args struct {
		initdata   []cloudflare.UnvalidatedIngressRule
		updatedata []cloudflare.UnvalidatedIngressRule
	}
	tests := []struct {
		name string
		n    *OrderedMap
		args args
		want []cloudflare.UnvalidatedIngressRule
	}{
		{
			name: "Update without duplicate",
			n:    NewOrderedMap(3),
			args: args{
				initdata: []cloudflare.UnvalidatedIngressRule{
					{
						Hostname: "test1",
						Service:  "value1",
					},
					{
						Hostname: "test2",
						Service:  "value2",
					},
					{
						Hostname: "test3",
						Service:  "value3",
					},
				},
				updatedata: []cloudflare.UnvalidatedIngressRule{
					{
						Hostname: "test1",
						Service:  "value4",
					},
				},
			},
			want: []cloudflare.UnvalidatedIngressRule{
				{
					Hostname: "test1",
					Service:  "value4",
				},
				{
					Hostname: "test2",
					Service:  "value2",
				},
				{
					Hostname: "test3",
					Service:  "value3",
				},
			},
		},
		{
			name: "Update with duplicate",
			n:    NewOrderedMap(10),
			args: args{
				initdata: []cloudflare.UnvalidatedIngressRule{
					{
						Hostname: "test1",
						Service:  "value1",
					},
					{
						Hostname: "test2",
						Service:  "value2",
					},
					{
						Hostname: "test3",
						Service:  "value3",
					},
				},
				updatedata: []cloudflare.UnvalidatedIngressRule{
					{
						Hostname: "test1",
						Service:  "value4",
					},
					{
						Hostname: "test2",
						Service:  "value5",
					},
					{
						Hostname: "test1",
						Service:  "value6",
					},
				},
			},
			want: []cloudflare.UnvalidatedIngressRule{
				{
					Hostname: "test1",
					Service:  "value6",
				},
				{
					Hostname: "test2",
					Service:  "value5",
				},
				{
					Hostname: "test3",
					Service:  "value3",
				},
			},
		},
		{
			name: "Update with new",
			n:    NewOrderedMap(10),
			args: args{
				initdata: []cloudflare.UnvalidatedIngressRule{
					{
						Hostname: "test1",
						Service:  "value1",
					},
					{
						Hostname: "test2",
						Service:  "value2",
					},
					{
						Hostname: "test3",
						Service:  "value3",
					},
				},
				updatedata: []cloudflare.UnvalidatedIngressRule{
					{
						Hostname: "test1",
						Service:  "value4",
					},

					{
						Hostname: "test4",
						Service:  "value4",
					},
				},
			},
			want: []cloudflare.UnvalidatedIngressRule{
				{
					Hostname: "test1",
					Service:  "value4",
				},
				{
					Hostname: "test2",
					Service:  "value2",
				},
				{
					Hostname: "test3",
					Service:  "value3",
				},
				{
					Hostname: "test4",
					Service:  "value4",
				},
			},
		},
	}
	for _, tt := range tests {
		for _, s := range tt.args.initdata {
			tt.n.Add(s)
		}
		t.Run(tt.name, func(t *testing.T) {
			for _, s := range tt.args.updatedata {
				tt.n.Update(s)
			}
			values := tt.n.Get()
			if len(values) != len(tt.want) {
				t.Errorf("Expected %d, got %d", len(tt.want), len(values))
			}
			if !reflect.DeepEqual(values, tt.want) {
				t.Errorf("Expected %v, got %v", tt.want, values)
			}
		})
	}
}

func TestOrderedMap_Remove(t *testing.T) {
	type args struct {
		initdata   []cloudflare.UnvalidatedIngressRule
		removedata []cloudflare.UnvalidatedIngressRule
	}
	tests := []struct {
		name string
		n    *OrderedMap
		args args
		want []cloudflare.UnvalidatedIngressRule
	}{
		{
			name: "Remove without duplicate",
			n:    NewOrderedMap(3),
			args: args{
				initdata: []cloudflare.UnvalidatedIngressRule{
					{
						Hostname: "test1",
						Service:  "value1",
					},
					{
						Hostname: "test2",
						Service:  "value2",
					},
					{
						Hostname: "test3",
						Service:  "value3",
					},
				},
				removedata: []cloudflare.UnvalidatedIngressRule{
					{
						Hostname: "test1",
						Service:  "value1",
					},
				},
			},

			want: []cloudflare.UnvalidatedIngressRule{
				{
					Hostname: "test2",
					Service:  "value2",
				},
				{
					Hostname: "test3",
					Service:  "value3",
				},
			},
		},
		{
			name: "Remove with duplicate",
			n:    NewOrderedMap(10),
			args: args{
				initdata: []cloudflare.UnvalidatedIngressRule{
					{
						Hostname: "test1",

						Service: "value1",
					},
					{
						Hostname: "test2",
						Service:  "value2",
					},
					{
						Hostname: "test3",
						Service:  "value3",
					},
				},
				removedata: []cloudflare.UnvalidatedIngressRule{
					{
						Hostname: "test1",
						Service:  "value1",
					},
					{
						Hostname: "test2",
						Service:  "value2",
					},
					{
						Hostname: "test1",
						Service:  "value1",
					},
				},
			},
			want: []cloudflare.UnvalidatedIngressRule{
				{
					Hostname: "test3",
					Service:  "value3",
				},
			},
		},
	}
	for _, tt := range tests {
		for _, s := range tt.args.initdata {
			tt.n.Add(s)
		}
		t.Run(tt.name, func(t *testing.T) {
			for _, s := range tt.args.removedata {
				tt.n.Remove(s)
			}
			values := tt.n.Get()
			if len(values) != len(tt.want) {
				t.Errorf("Expected %d, got %d", len(tt.want), len(values))
			}
			if !reflect.DeepEqual(values, tt.want) {
				t.Errorf("Expected %v, got %v", tt.want, values)
			}
		})
	}
}
