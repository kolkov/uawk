package vm

import (
	"strings"
	"testing"
)

var benchStrings = []string{
	"hello world",
	"HELLO WORLD",
	"Hello World",
	"ThIs Is A MiXeD CaSe StRiNg WiTh MaNy WoRdS",
	"already lowercase string with no changes needed",
	"ALREADY UPPERCASE STRING WITH NO CHANGES NEEDED",
	"123 numbers 456 and 789 symbols !@#$%",
}

func BenchmarkToLowerASCII(b *testing.B) {
	for _, s := range benchStrings {
		name := s
		if len(name) > 20 {
			name = name[:20]
		}
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = toLowerASCII(s)
			}
		})
	}
}

func BenchmarkToLowerStdlib(b *testing.B) {
	for _, s := range benchStrings {
		name := s
		if len(name) > 20 {
			name = name[:20]
		}
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = strings.ToLower(s)
			}
		})
	}
}

func BenchmarkToUpperASCII(b *testing.B) {
	for _, s := range benchStrings {
		name := s
		if len(name) > 20 {
			name = name[:20]
		}
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = toUpperASCII(s)
			}
		})
	}
}

func BenchmarkToUpperStdlib(b *testing.B) {
	for _, s := range benchStrings {
		name := s
		if len(name) > 20 {
			name = name[:20]
		}
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = strings.ToUpper(s)
			}
		})
	}
}
