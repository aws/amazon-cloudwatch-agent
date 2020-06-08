package awscsm

import (
	"testing"
)

func TestGenerateUIDObjects(t *testing.T) {
	type objFoo struct {
		Foo int
	}

	type objBar struct {
		Bar string
	}

	cases := []struct {
		obj1         interface{}
		obj2         interface{}
		expectedSame bool
	}{
		{
			obj1: map[string]string{
				"Foo": "123",
				"Bar": "456",
				"Baz": "000",
				"Qux": "qux",
			},
			obj2: map[string]string{
				"Bar": "456",
				"Foo": "123",
				"Qux": "qux",
				"Baz": "000",
			},
			expectedSame: true,
		},
		{
			obj1: map[string]string{
				"Foo": "123",
				"Bar": "456",
				"Baz": "000",
				"Qux": "qux",
			},
			obj2: map[string]string{
				"Bar": "456",
				"Foo": "123",
				"Qux": "qux",
			},
			expectedSame: false,
		},
		{
			obj1:         0,
			obj2:         "bar",
			expectedSame: false,
		},
		{
			obj1:         objFoo{},
			obj2:         objBar{},
			expectedSame: false,
		},
		{
			obj1:         objFoo{},
			obj2:         objFoo{},
			expectedSame: true,
		},
		{
			obj1:         objFoo{1},
			obj2:         objBar{"1"},
			expectedSame: false,
		},
		{
			obj1:         objFoo{1},
			obj2:         objFoo{1},
			expectedSame: true,
		},
		{
			obj1:         objFoo{1},
			obj2:         objFoo{2},
			expectedSame: false,
		},
	}

	for i, c := range cases {
		uid1, err := generateUID(c.obj1)
		if err != nil {
			t.Errorf("%d: expected no error, but received %v", i, err)
		}

		uid2, err := generateUID(c.obj2)
		if err != nil {
			t.Errorf("%d: expected no error, but received %v", i, err)
		}

		same := uid1 == uid2
		if same != c.expectedSame {
			t.Errorf("%d: expected %t, but received %t", i, c.expectedSame, same)
		}
	}
}
