//go:build darwin || linux

package json

import (
	"syscall"
	"testing"
)

func TestStringScannerGuardPage(t *testing.T) {
	page := syscall.Getpagesize()
	memory, err := syscall.Mmap(-1, 0, page*3, syscall.PROT_NONE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := syscall.Munmap(memory); err != nil {
			t.Error(err)
		}
	}()

	middle := memory[page : 2*page]
	if err := syscall.Mprotect(middle, syscall.PROT_READ|syscall.PROT_WRITE); err != nil {
		t.Fatal(err)
	}

	for i := range middle {
		middle[i] = 'a'
	}

	protect := func(mode int) {
		if err := syscall.Mprotect(middle, mode); err != nil {
			t.Fatal(err)
		}
	}
	protect(syscall.PROT_READ)

	sizes := []int{255, 256, 257, 4095, 4096, 4097, page - 1, page}
	for size := 0; size <= 128; size++ {
		sizes = append(sizes, size)
	}

	for _, size := range sizes {
		if size > page {
			continue
		}

		for _, raw := range [][]byte{middle[:size], middle[page-size:]} {
			if got := indexStringSpecial(raw); got != size {
				t.Fatalf("size=%d got=%d", size, got)
			}

			if size > 0 {
				protect(syscall.PROT_READ | syscall.PROT_WRITE)
				raw[size-1] = '"'
				protect(syscall.PROT_READ)
				if got := indexStringSpecial(raw); got != size-1 {
					t.Fatalf("tail: size=%d got=%d", size, got)
				}

				protect(syscall.PROT_READ | syscall.PROT_WRITE)
				raw[size-1] = 'a'
				protect(syscall.PROT_READ)
			}
		}
	}
}
