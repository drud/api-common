package state

import "testing"

func TestTransform(t *testing.T) {

	type StripeNested struct {
		Element2 string `json:"element2,omitempty"`
	}
	type StripeObject struct {
		Element1 string       `json:"element1,omitempty"`
		Nested   StripeNested `json:"nested1,omitempty"`
	}

	type ConvertType struct {
		Element1 string `json:"element1" transform:".element1"`
		Element2 string `json:"element2" transform:".nested1.element2"`
	}

	type args struct {
		stripeObj interface{}
		obj       interface{}
	}
	test := &ConvertType{}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "testset",
			args: args{
				stripeObj: &StripeObject{
					Element1: "data1",
					Nested: StripeNested{
						Element2: "data2",
					},
				},
				obj: test,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Transform(tt.args.stripeObj, tt.args.obj); (err != nil) != tt.wantErr {
				t.Errorf("Transform() error = %v, wantErr %v", err, tt.wantErr)
			}
			if test.Element1 != "data1" {
				t.Errorf("Transform fail Element1")
			}
			if test.Element2 != "data2" {
				t.Errorf("Transform fail Element2")
			}
		})
	}
}
