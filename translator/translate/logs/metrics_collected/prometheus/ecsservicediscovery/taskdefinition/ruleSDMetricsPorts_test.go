package taskdefinition

import "testing"

func Test_checkMetricPortString(t *testing.T) {
	type args struct {
		portsConfig string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "invalid_test_bad_separator",
			args: args{portsConfig: "1,2,3,4"},
			want: false},
		{name: "invalid_test_port_too_big",
			args: args{portsConfig: "12321321"},
			want: false},
		{name: "invalid_test_alphabet_not_allow",
			args: args{portsConfig: "9901; a;123"},
			want: false},
		{name: "invalid_test_bad_ending",
			args: args{portsConfig: "9901;  123;"},
			want: false},
		{name: "invalid_test_empty_not_allow",
			args: args{portsConfig: ""},
			want: false},
		{name: "invalid_test_zero_not_allow",
			args: args{portsConfig: "1; 0"},
			want: false},
		{name: "valid_test_space_allowed",
			args: args{portsConfig: "9404; 9406;		9901;   9123"},
			want: true},
		{name: "valid_test_normal_case",
			args: args{portsConfig: "19404"},
			want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkMetricPortString(tt.args.portsConfig); got != tt.want {
				t.Errorf("checkMetricPortString() = %v, want %v", got, tt.want)
			}
		})
	}
}
