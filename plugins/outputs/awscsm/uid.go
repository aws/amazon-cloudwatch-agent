// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsm

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"reflect"
	"sort"
)

func generateUID(m interface{}) (string, error) {
	buf := serialize(reflect.ValueOf(m))
	b := sha1.Sum(buf)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func serialize(v reflect.Value) []byte {
	buf := bytes.Buffer{}

	switch v.Kind() {
	case reflect.Ptr:
		return serialize(v.Elem())
	case reflect.Struct:
		t := v.Type()
		buf.WriteString(fmt.Sprintf("<%s>", t.Kind()))
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if !field.CanInterface() {
				continue
			}
			buf.WriteString(fmt.Sprintf("%s<%s>:", t.Field(i).Name, field.Kind()))
			buf.Write(serialize(field))
		}
	case reflect.Map:
		keys := v.MapKeys()
		sort.Sort(sortReflectValues(keys))
		buf.WriteString("<map>")
		for _, key := range keys {
			buf.WriteString(fmt.Sprintf("%s<%s>:", key.String(), key.Kind()))
			buf.Write(serialize(v.MapIndex(key)))
		}
	case reflect.Array, reflect.Slice:
		buf.WriteString("<array>")
		for i := 0; i < v.Len(); i++ {
			buf.WriteString(fmt.Sprintf("%d:", i))
			buf.Write(serialize(v.Index(i)))
		}
	default:
		buf.WriteString(fmt.Sprintf("<%s>%v", v.Kind(), v.Interface()))
	}

	return buf.Bytes()
}

type sortReflectValues []reflect.Value

func (v sortReflectValues) Len() int           { return len([]reflect.Value(v)) }
func (v sortReflectValues) Less(i, j int) bool { return v[i].String() < v[j].String() }
func (v sortReflectValues) Swap(i, j int)      { v[i], v[j] = v[j], v[i] }
