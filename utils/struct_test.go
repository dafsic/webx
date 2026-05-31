package utils

import (
	"testing"
)

// 测试用的结构体
type TestStruct struct {
	Name    string  `json:"name"`
	Age     int     `json:"age"`
	Score   float64 `json:"score"`
	Enabled bool    `json:"enabled"`
	NoTag   string  // 没有 target tag
}

type PointerStruct struct {
	ID       *int64   `json:"id"`
	Username *string  `json:"username"`
	Active   *bool    `json:"active"`
	Price    *float64 `json:"price"`
}

func TestStructToMap(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected map[string]any
	}{
		{
			name: "基本类型转换",
			input: &TestStruct{
				Name:    "Alice",
				Age:     25,
				Score:   95.5,
				Enabled: true,
				NoTag:   "ignored",
			},
			expected: map[string]any{
				"name":    "Alice",
				"age":     25,
				"score":   95.5,
				"enabled": true,
				"":        "ignored", // 没有tag的字段，key为空字符串
			},
		},
		{
			name: "零值转换",
			input: &TestStruct{
				Name:    "",
				Age:     0,
				Score:   0.0,
				Enabled: false,
			},
			expected: map[string]any{
				"name":    "",
				"age":     0,
				"score":   0.0,
				"enabled": false,
				"":        "",
			},
		},
		{
			name: "指针类型转换",
			input: &PointerStruct{
				ID:       Pointer(int64(12345)),
				Username: Pointer("testuser"),
				Active:   Pointer(true),
				Price:    Pointer(99.99),
			},
			expected: map[string]any{
				"id":       Pointer(int64(12345)),
				"username": Pointer("testuser"),
				"active":   Pointer(true),
				"price":    Pointer(99.99),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StructToMap(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("StructToMap() map length = %d, want %d", len(result), len(tt.expected))
				return
			}

			for key, expectedVal := range tt.expected {
				actualVal, ok := result[key]
				if !ok {
					t.Errorf("StructToMap() missing key %q", key)
					continue
				}

				// 使用类型断言比较指针值
				if !compareValues(actualVal, expectedVal) {
					t.Errorf("StructToMap() key %q = %v, want %v", key, actualVal, expectedVal)
				}
			}
		})
	}
}

func TestMapToStruct(t *testing.T) {
	tests := []struct {
		name      string
		input     map[string]any
		output    any
		expected  any
		shouldErr bool
	}{
		{
			name: "基本类型转换",
			input: map[string]any{
				"name":    "Bob",
				"age":     30,
				"score":   88.5,
				"enabled": true,
			},
			output: &TestStruct{},
			expected: &TestStruct{
				Name:    "Bob",
				Age:     30,
				Score:   88.5,
				Enabled: true,
			},
			shouldErr: false,
		},
		{
			name: "部分字段转换",
			input: map[string]any{
				"name": "Charlie",
				"age":  40,
			},
			output: &TestStruct{},
			expected: &TestStruct{
				Name:    "Charlie",
				Age:     40,
				Score:   0.0,
				Enabled: false,
			},
			shouldErr: false,
		},
		{
			name: "类型转换 - float64 to int",
			input: map[string]any{
				"name": "David",
				"age":  float64(35), // 模拟 JSON 解析后的数字类型
			},
			output: &TestStruct{},
			expected: &TestStruct{
				Name: "David",
				Age:  35,
			},
			shouldErr: false,
		},
		{
			name: "忽略没有 tag 的字段",
			input: map[string]any{
				"name":  "Eve",
				"extra": "ignored",
			},
			output: &TestStruct{},
			expected: &TestStruct{
				Name: "Eve",
			},
			shouldErr: false,
		},
		{
			name: "非指针输入应该报错",
			input: map[string]any{
				"name": "Error",
			},
			output:    TestStruct{}, // 不是指针
			expected:  nil,
			shouldErr: true,
		},
		{
			name: "nil 指针应该报错",
			input: map[string]any{
				"name": "Error",
			},
			output:    (*TestStruct)(nil),
			expected:  nil,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MapToStruct(tt.input, tt.output)

			if tt.shouldErr {
				if err == nil {
					t.Error("MapToStruct() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("MapToStruct() unexpected error: %v", err)
				return
			}

			if tt.expected != nil {
				if !compareStructs(tt.output, tt.expected) {
					t.Errorf("MapToStruct() = %+v, want %+v", tt.output, tt.expected)
				}
			}
		})
	}
}

func TestStructToMapAndBack(t *testing.T) {
	// 测试往返转换
	original := &TestStruct{
		Name:    "RoundTrip",
		Age:     50,
		Score:   100.0,
		Enabled: true,
	}

	// Struct -> Map
	m := StructToMap(original)

	// Map -> Struct
	result := &TestStruct{}
	err := MapToStruct(m, result)
	if err != nil {
		t.Fatalf("MapToStruct() error: %v", err)
	}

	// 比较结果（忽略 NoTag 字段，因为它没有 target tag）
	if result.Name != original.Name {
		t.Errorf("Name = %v, want %v", result.Name, original.Name)
	}
	if result.Age != original.Age {
		t.Errorf("Age = %v, want %v", result.Age, original.Age)
	}
	if result.Score != original.Score {
		t.Errorf("Score = %v, want %v", result.Score, original.Score)
	}
	if result.Enabled != original.Enabled {
		t.Errorf("Enabled = %v, want %v", result.Enabled, original.Enabled)
	}
}

// 辅助函数：比较两个值
func compareValues(a, b any) bool {
	// 处理指针类型
	if ptrA, ok := a.(*int64); ok {
		if ptrB, ok := b.(*int64); ok {
			if ptrA == nil && ptrB == nil {
				return true
			}
			if ptrA != nil && ptrB != nil {
				return *ptrA == *ptrB
			}
			return false
		}
	}
	if ptrA, ok := a.(*string); ok {
		if ptrB, ok := b.(*string); ok {
			if ptrA == nil && ptrB == nil {
				return true
			}
			if ptrA != nil && ptrB != nil {
				return *ptrA == *ptrB
			}
			return false
		}
	}
	if ptrA, ok := a.(*bool); ok {
		if ptrB, ok := b.(*bool); ok {
			if ptrA == nil && ptrB == nil {
				return true
			}
			if ptrA != nil && ptrB != nil {
				return *ptrA == *ptrB
			}
			return false
		}
	}
	if ptrA, ok := a.(*float64); ok {
		if ptrB, ok := b.(*float64); ok {
			if ptrA == nil && ptrB == nil {
				return true
			}
			if ptrA != nil && ptrB != nil {
				return *ptrA == *ptrB
			}
			return false
		}
	}

	// 基本类型比较
	return a == b
}

// 辅助函数：比较两个结构体
func compareStructs(a, b any) bool {
	if ts1, ok := a.(*TestStruct); ok {
		if ts2, ok := b.(*TestStruct); ok {
			return ts1.Name == ts2.Name &&
				ts1.Age == ts2.Age &&
				ts1.Score == ts2.Score &&
				ts1.Enabled == ts2.Enabled
		}
	}
	return false
}
