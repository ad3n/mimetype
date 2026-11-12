//go:build !purego

#include "textflag.h"

TEXT ·textBytes(SB), NOSPLIT, $0-25
	MOVD raw_base+0(FP), R0
	MOVD raw_len+8(FP), R1
	CMP $16, R1
	BLO scalar

	MOVD $textControl<>(SB), R2
	VLD1 (R2), [V0.B16, V1.B16]
	ADD R0, R1, R2
	SUB $16, R2, R2

// Table lookup yields zero for all bytes >= 32 and allowed control bytes.
vector:
	VLD1 (R0), [V2.B16]
	VTBL V2.B16, [V0.B16, V1.B16], V3.B16
	VUMAXV V3.B16, V3
	VMOV V3.B[0], R3
	CBNZ R3, invalid
	CMP R2, R0
	BEQ valid
	ADD $16, R0, R0
	CMP R2, R0
	BLS vector
	// Recheck the final 16 bytes without reading beyond the slice.
	MOVD R2, R0
	B vector

scalar:
	CBZ R1, valid
	MOVBU (R0), R3
	CMP $32, R3
	BHS scalarNext
	MOVD $0xf7ffc9ff, R4
	LSR R3, R4, R5
	TBNZ $0, R5, invalid

scalarNext:
	ADD $1, R0, R0
	SUB $1, R1, R1
	B scalar

valid:
	MOVD $1, R0
	MOVB R0, ret+24(FP)
	RET

invalid:
	MOVB ZR, ret+24(FP)
	RET

DATA textControl<>+0(SB)/8, $0xffffffffffffffff
DATA textControl<>+8(SB)/8, $0xffff0000ff0000ff
DATA textControl<>+16(SB)/8, $0xffffffffffffffff
DATA textControl<>+24(SB)/8, $0xffffffff00ffffff
GLOBL textControl<>(SB), RODATA|NOPTR, $32
