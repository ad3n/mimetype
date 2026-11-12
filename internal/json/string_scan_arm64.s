//go:build !purego

#include "textflag.h"

TEXT ·indexStringSpecialNEON(SB), NOSPLIT, $0-32
	MOVD raw_base+0(FP), R0
	MOVD raw_len+8(FP), R1
	MOVD R0, R2
	MOVD $34, R3
	VDUP R3, V0.B16
	MOVD $92, R3
	VDUP R3, V1.B16

vector:
	CMP $16, R1
	BLO scalar
	VLD1 (R0), [V2.B16]
	VCMEQ V0.B16, V2.B16, V3.B16
	VCMEQ V1.B16, V2.B16, V4.B16
	VORR V3.B16, V4.B16, V3.B16
	VMOV V3.D[0], R3
	CBNZ R3, foundVector
	VMOV V3.D[1], R3
	CBNZ R3, foundUpper
	ADD $16, R0, R0
	SUB $16, R1, R1
	B vector

foundUpper:
	ADD $8, R0, R0

foundVector:
	RBIT R3, R3
	CLZ R3, R3
	ADD R3>>3, R0, R0
	B done

scalar:
	CBZ R1, done
	MOVBU (R0), R3
	CMP $34, R3
	BEQ done
	CMP $92, R3
	BEQ done
	ADD $1, R0, R0
	SUB $1, R1, R1
	B scalar

done:
	SUB R2, R0, R0
	MOVD R0, ret+24(FP)
	RET
