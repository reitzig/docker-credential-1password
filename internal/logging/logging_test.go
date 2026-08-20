package logging

import "testing"

func Test_isEnabled(t *testing.T) {
	type args struct {
		value string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "one",
			args: args{value: "1"},
			want: true,
		},
		{
			name: "true",
			args: args{value: "true"},
			want: true,
		},
		{
			name: "yes",
			args: args{value: "yes"},
			want: true,
		},
		{
			name: "on",
			args: args{value: "on"},
			want: true,
		},
		{
			name: "uppercase truthy value",
			args: args{value: "TRUE"},
			want: true,
		},
		{
			name: "empty",
			args: args{value: ""},
			want: false,
		},
		{
			name: "zero",
			args: args{value: "0"},
			want: false,
		},
		{
			name: "false",
			args: args{value: "false"},
			want: false,
		},
		{
			name: "no",
			args: args{value: "no"},
			want: false,
		},
		{
			name: "off",
			args: args{value: "off"},
			want: false,
		},
		{
			name: "unrecognized value",
			args: args{value: "maybe"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDebugEnabled(tt.args.value); got != tt.want {
				t.Errorf("isEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
