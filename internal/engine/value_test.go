package engine

import "testing"

func TestValidateValue(t *testing.T) {
	min, max := 0.0, 100.0
	cases := []struct {
		name    string
		spec    VarSpec
		val     any
		wantErr bool
	}{
		{"int ok", VarSpec{Type: "int", Min: &min, Max: &max}, float64(50), false},
		{"int below min", VarSpec{Type: "int", Min: &min, Max: &max}, float64(-1), true},
		{"int not whole", VarSpec{Type: "int"}, 1.5, true},
		{"float ok", VarSpec{Type: "float"}, 3.14, false},
		{"bool ok", VarSpec{Type: "bool"}, true, false},
		{"bool wrong", VarSpec{Type: "bool"}, "yes", true},
		{"string ok", VarSpec{Type: "string"}, "hi", false},
		{"enum ok", VarSpec{Type: "enum", Values: []string{"a", "b"}}, "a", false},
		{"enum bad", VarSpec{Type: "enum", Values: []string{"a", "b"}}, "c", true},
		{"ref is string", VarSpec{Type: "ref", RefType: "location"}, "vault", false},
		{"ref non-string", VarSpec{Type: "ref"}, 3, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateValue(c.spec, c.val)
			if (err != nil) != c.wantErr {
				t.Fatalf("ValidateValue(%v)=%v, wantErr=%v", c.val, err, c.wantErr)
			}
		})
	}
}

func TestDefaultValue(t *testing.T) {
	if got := DefaultValue(VarSpec{Type: "int"}); got != float64(0) {
		t.Errorf("int default = %v", got)
	}
	if got := DefaultValue(VarSpec{Type: "enum", Values: []string{"x", "y"}}); got != "x" {
		t.Errorf("enum default = %v", got)
	}
	if got := DefaultValue(VarSpec{Type: "int", Default: float64(5)}); got != float64(5) {
		t.Errorf("explicit default = %v", got)
	}
}

func TestSetValidateValue(t *testing.T) {
	spec := VarSpec{Type: "set", Elem: "string"}
	cases := []struct {
		name    string
		val     any
		wantErr bool
	}{
		{"empty set ok", []any{}, false},
		{"set with strings ok", []any{"a", "b", "c"}, false},
		{"set not array", "oops", true},
		{"set element not string", []any{"a", float64(1)}, true},
		{"set duplicate element", []any{"a", "b", "a"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateValue(spec, c.val)
			if (err != nil) != c.wantErr {
				t.Fatalf("ValidateValue(%v)=%v, wantErr=%v", c.val, err, c.wantErr)
			}
		})
	}
}

func TestSetDefaultValue(t *testing.T) {
	got := DefaultValue(VarSpec{Type: "set", Elem: "string"})
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("set default should be []any, got %T", got)
	}
	if len(arr) != 0 {
		t.Fatalf("set default should be empty, got %v", arr)
	}
}
