package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"unsafe"

	"github.com/google/uuid"
)

// UUID generates a new UUID (Universally Unique Identifier).
func UUID() string {
	return uuid.New().String()
}

// Pointer returns a pointer to the value of the given type.
func Pointer[T any](val T) *T {
	return &val
}

// StringToBytes converts a string to a byte slice without copying the data.
func StringToBytes(s string) []byte {
	stringHeader := unsafe.StringData(s)
	return unsafe.Slice(stringHeader, len(s))
}

// BytesToString converts a byte slice to a string without copying the data.
func BytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// StrSplit splits a string by commas or semicolons and returns a slice of strings.
func StrSplit(s string) []string {
	return strings.FieldsFunc(s, func(c rune) bool { return c == ',' || c == ';' })
}

// ConcatStrings concatenates multiple strings into a single slice.
func ConcatStrings(elems ...string) []string {
	return elems
}

func ConcatSlices[T any](slices ...[]T) []T {
	var totalLen int
	for _, slice := range slices {
		totalLen += len(slice)
	}

	result := make([]T, 0, totalLen)
	for _, slice := range slices {
		result = append(result, slice...)
	}
	return result
}

// UniqueStrings 合并多个字符串切片并去重，保持首次出现的顺序
func UniqueStrings(slices ...[]string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, s := range slices {
		for _, v := range s {
			if _, ok := seen[v]; !ok {
				seen[v] = struct{}{}
				result = append(result, v)
			}
		}
	}
	return result
}

// CompressStr removes all whitespace characters from a string.
func CompressStr(str string) string {
	if str == "" {
		return ""
	}
	reg := regexp.MustCompile(`\\s+`) // \s matches any whitespace character (space, tab, newline, etc.)
	return reg.ReplaceAllString(str, "")
}

func StringToInt32(numStr string) int32 {
	num, _ := strconv.Atoi(numStr)
	return int32(num)
}

func StringToFloat64(num string) float64 {
	fnum, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0
	}
	return fnum
}

func Float64ToString(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func FormatFloat(value float64, precision float64) float64 {
	precision = math.Pow(10, precision)
	return math.Round(value*precision) / precision
}

// GenerateRandomString generates a random string of the specified length.
func GenerateRandomString(length int) string {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

// 生成随机数只有数字
func GenerateRandomNumeric(length int) string {
	num := ""
	for i := 0; i < length; i++ {
		num += strconv.Itoa(rand.Intn(10))
	}
	return num
}

// LineInfo returns the function name, file name and line number of the caller function.
func LineInfo() string {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "???"
		line = 0
	}
	//function := runtime.FuncForPC(pc).Name()

	return fmt.Sprintf(" <<< %s:%d", file, line)
	//return fmt.Sprintf("\n%s\n\t%s:%d", function, file, line)
	//return strings.Join(ConcatStrings("\n\t", file, ":", strconv.Itoa(line)), "")
}

func MD5(v string) string {
	if v == "" {
		return ""
	}
	hash := md5.Sum(StringToBytes(v))
	return hex.EncodeToString(hash[:])
}

// randomFloat generates a random float between min and max.
func RandomFloat(min, max float64) float64 {
	return min + (max-min)*rand.Float64()
}
