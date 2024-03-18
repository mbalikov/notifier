package main

import (
	"syscall"
)

const NBITS = len(syscall.FdSet{}.Bits)

func fdset_ZERO(fdset *syscall.FdSet) {
	for i := range len(fdset.Bits) {
		fdset.Bits[i] = 0
	}
}

func fdset_Set(fdset *syscall.FdSet, fd int) {
	fdset.Bits[fd/NBITS] |= 1 << (fd % NBITS)
}

func fdset_Clear(fdset *syscall.FdSet, fd int) {
	fdset.Bits[fd/NBITS] &= ^(1 << (fd % NBITS))
}

func fdset_IsSet(fdset *syscall.FdSet, fd int) bool {
	return (fdset.Bits[fd/NBITS] & (1 << (fd % NBITS))) != 0
}
