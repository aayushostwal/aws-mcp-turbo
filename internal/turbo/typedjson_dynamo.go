package turbo

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type memberBuilder func(v any, path string) (reflect.Value, error)

func strList(v any, path string) ([]string, error) {
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: expected an array of strings", path)
	}
	out := make([]string, len(arr))
	for i, e := range arr {
		switch x := e.(type) {
		case string:
			out[i] = x
		case json.Number:
			out[i] = x.String()
		default:
			return nil, fmt.Errorf("%s[%d]: expected a string", path, i)
		}
	}
	return out, nil
}

var dynamoMembers map[string]memberBuilder

func init() {
	dynamoMembers = map[string]memberBuilder{
		"S": func(v any, path string) (reflect.Value, error) {
			s, ok := v.(string)
			if !ok {
				return reflect.Value{}, fmt.Errorf("%s: S must be a string", path)
			}
			return reflect.ValueOf(&types.AttributeValueMemberS{Value: s}), nil
		},
		// N is a string in DynamoDB; a bare JSON number is accepted for convenience.
		"N": func(v any, path string) (reflect.Value, error) {
			switch x := v.(type) {
			case string:
				return reflect.ValueOf(&types.AttributeValueMemberN{Value: x}), nil
			case json.Number:
				return reflect.ValueOf(&types.AttributeValueMemberN{Value: x.String()}), nil
			}
			return reflect.Value{}, fmt.Errorf("%s: N must be a number string", path)
		},
		"B": func(v any, path string) (reflect.Value, error) {
			s, _ := v.(string)
			b, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("%s: B must be base64", path)
			}
			return reflect.ValueOf(&types.AttributeValueMemberB{Value: b}), nil
		},
		"BOOL": func(v any, path string) (reflect.Value, error) {
			b, ok := v.(bool)
			if !ok {
				return reflect.Value{}, fmt.Errorf("%s: BOOL must be true or false", path)
			}
			return reflect.ValueOf(&types.AttributeValueMemberBOOL{Value: b}), nil
		},
		"NULL": func(v any, path string) (reflect.Value, error) {
			return reflect.ValueOf(&types.AttributeValueMemberNULL{Value: true}), nil
		},
		"SS": func(v any, path string) (reflect.Value, error) {
			l, err := strList(v, path)
			return reflect.ValueOf(&types.AttributeValueMemberSS{Value: l}), err
		},
		"NS": func(v any, path string) (reflect.Value, error) {
			l, err := strList(v, path)
			return reflect.ValueOf(&types.AttributeValueMemberNS{Value: l}), err
		},
		"BS": func(v any, path string) (reflect.Value, error) {
			l, err := strList(v, path)
			var out [][]byte
			for _, s := range l {
				b, e := base64.StdEncoding.DecodeString(s)
				if e != nil {
					return reflect.Value{}, fmt.Errorf("%s: BS must be base64", path)
				}
				out = append(out, b)
			}
			return reflect.ValueOf(&types.AttributeValueMemberBS{Value: out}), err
		},
		"L": func(v any, path string) (reflect.Value, error) {
			arr, ok := v.([]any)
			if !ok {
				return reflect.Value{}, fmt.Errorf("%s: L must be an array", path)
			}
			out := make([]types.AttributeValue, len(arr))
			for i, e := range arr {
				av, err := attributeValue(e, nil, fmt.Sprintf("%s[%d]", path, i))
				if err != nil {
					return reflect.Value{}, err
				}
				out[i] = av.Interface().(types.AttributeValue)
			}
			return reflect.ValueOf(&types.AttributeValueMemberL{Value: out}), nil
		},
		"M": func(v any, path string) (reflect.Value, error) {
			obj, ok := v.(map[string]any)
			if !ok {
				return reflect.Value{}, fmt.Errorf("%s: M must be an object", path)
			}
			out := make(map[string]types.AttributeValue, len(obj))
			for k, e := range obj {
				av, err := attributeValue(e, nil, path+"."+k)
				if err != nil {
					return reflect.Value{}, err
				}
				out[k] = av.Interface().(types.AttributeValue)
			}
			return reflect.ValueOf(&types.AttributeValueMemberM{Value: out}), nil
		},
	}
}

// avWire renders an attribute value in DynamoDB's JSON form so that a
// continuation key can be passed back as a request parameter unchanged.
func avWire(av types.AttributeValue) any {
	switch m := av.(type) {
	case *types.AttributeValueMemberS:
		return map[string]any{"S": m.Value}
	case *types.AttributeValueMemberN:
		return map[string]any{"N": m.Value}
	case *types.AttributeValueMemberB:
		return map[string]any{"B": base64.StdEncoding.EncodeToString(m.Value)}
	case *types.AttributeValueMemberBOOL:
		return map[string]any{"BOOL": m.Value}
	case *types.AttributeValueMemberNULL:
		return map[string]any{"NULL": true}
	case *types.AttributeValueMemberSS:
		return map[string]any{"SS": m.Value}
	case *types.AttributeValueMemberNS:
		return map[string]any{"NS": m.Value}
	case *types.AttributeValueMemberBS:
		enc := make([]string, len(m.Value))
		for i, b := range m.Value {
			enc[i] = base64.StdEncoding.EncodeToString(b)
		}
		return map[string]any{"BS": enc}
	case *types.AttributeValueMemberL:
		out := make([]any, len(m.Value))
		for i, e := range m.Value {
			out[i] = avWire(e)
		}
		return map[string]any{"L": out}
	case *types.AttributeValueMemberM:
		out := make(map[string]any, len(m.Value))
		for k, e := range m.Value {
			out[k] = avWire(e)
		}
		return map[string]any{"M": out}
	}
	return nil
}

func keyWire(key map[string]types.AttributeValue) map[string]any {
	out := make(map[string]any, len(key))
	for k, v := range key {
		out[k] = avWire(v)
	}
	return out
}
