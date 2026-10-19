package turbo

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// decodeTyped fills an SDK input struct from JSON. It exists for inputs that
// encoding/json cannot build: DynamoDB attribute values are an interface type,
// so they are written in DynamoDB's own JSON form ({"S":"x"}, {"N":"1"}, ...).
// Behaviour for every other field matches encoding/json with exact field names.
func decodeTyped(data []byte, target any) error {
	var src any
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.UseNumber()
	if err := d.Decode(&src); err != nil {
		return err
	}
	return fillValue(reflect.ValueOf(target).Elem(), src, "")
}

func fillValue(dst reflect.Value, src any, path string) error {
	if src == nil {
		return nil
	}
	t := dst.Type()
	if t.Kind() == reflect.Interface {
		if t.Name() == "AttributeValue" && strings.HasSuffix(t.PkgPath(), "dynamodb/types") {
			av, err := attributeValue(src, t, path)
			if err != nil {
				return err
			}
			dst.Set(av)
			return nil
		}
		return fmt.Errorf("%s: unsupported union type %s", path, t.Name())
	}
	if t.Kind() == reflect.Pointer {
		v := reflect.New(t.Elem())
		if err := fillValue(v.Elem(), src, path); err != nil {
			return err
		}
		dst.Set(v)
		return nil
	}
	switch t.Kind() {
	case reflect.Struct:
		if t == reflect.TypeFor[time.Time]() {
			s, ok := src.(string)
			if !ok {
				return fmt.Errorf("%s: expected an RFC 3339 timestamp string", path)
			}
			tm, err := time.Parse(time.RFC3339, s)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			dst.Set(reflect.ValueOf(tm))
			return nil
		}
		obj, ok := src.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: expected an object", path)
		}
		for k, v := range obj {
			f, ok := t.FieldByName(k)
			if !ok || !f.IsExported() {
				return fmt.Errorf("%s: unknown field %q", path, k)
			}
			if err := fillValue(dst.FieldByIndex(f.Index), v, path+"."+k); err != nil {
				return err
			}
		}
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			s, ok := src.(string)
			if !ok {
				return fmt.Errorf("%s: expected base64 string", path)
			}
			b, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			dst.SetBytes(b)
			return nil
		}
		arr, ok := src.([]any)
		if !ok {
			return fmt.Errorf("%s: expected an array", path)
		}
		out := reflect.MakeSlice(t, len(arr), len(arr))
		for i, e := range arr {
			if err := fillValue(out.Index(i), e, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
		dst.Set(out)
	case reflect.Map:
		obj, ok := src.(map[string]any)
		if !ok || t.Key().Kind() != reflect.String {
			return fmt.Errorf("%s: expected an object", path)
		}
		out := reflect.MakeMapWithSize(t, len(obj))
		for k, e := range obj {
			v := reflect.New(t.Elem()).Elem()
			if err := fillValue(v, e, path+"."+k); err != nil {
				return err
			}
			out.SetMapIndex(reflect.ValueOf(k).Convert(t.Key()), v)
		}
		dst.Set(out)
	case reflect.String:
		s, ok := src.(string)
		if !ok {
			return fmt.Errorf("%s: expected a string", path)
		}
		dst.SetString(s)
	case reflect.Bool:
		b, ok := src.(bool)
		if !ok {
			return fmt.Errorf("%s: expected a boolean", path)
		}
		dst.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, ok := src.(json.Number)
		if !ok {
			return fmt.Errorf("%s: expected a number", path)
		}
		i, err := strconv.ParseInt(n.String(), 10, 64)
		if err != nil || dst.OverflowInt(i) {
			return fmt.Errorf("%s: invalid integer %s", path, n)
		}
		dst.SetInt(i)
	case reflect.Float32, reflect.Float64:
		n, ok := src.(json.Number)
		if !ok {
			return fmt.Errorf("%s: expected a number", path)
		}
		f, err := n.Float64()
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		dst.SetFloat(f)
	default:
		return fmt.Errorf("%s: unsupported type %s", path, t)
	}
	return nil
}

// attributeValue converts DynamoDB JSON into the SDK's AttributeValue member
// types by looking them up on the interface's package.
func attributeValue(src any, iface reflect.Type, path string) (reflect.Value, error) {
	obj, ok := src.(map[string]any)
	if !ok || len(obj) != 1 {
		return reflect.Value{}, fmt.Errorf(`%s: attribute values look like {"S":"text"} or {"N":"1"}`, path)
	}
	for kind, v := range obj {
		member, ok := dynamoMembers[kind]
		if !ok {
			return reflect.Value{}, fmt.Errorf("%s: unknown attribute type %q (use S, N, B, BOOL, NULL, L, M, SS, NS, BS)", path, kind)
		}
		return member(v, path)
	}
	return reflect.Value{}, nil
}
