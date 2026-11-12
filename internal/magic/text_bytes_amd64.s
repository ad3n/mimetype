//go:build !purego

#include "textflag.h"

TEXT ·textBytes(SB), NOSPLIT, $0-25
	MOVQ raw_base+0(FP), AX
	MOVQ raw_len+8(FP), CX
	CMPQ CX, $16
	JB scalar

	MOVQ AX, DX
	ADDQ CX, DX
	SUBQ $16, DX
	MOVQ $0x2020202020202020, R8
	MOVQ R8, X1
	PUNPCKLQDQ X1, X1
	MOVQ $0x0909090909090909, R8
	MOVQ R8, X2
	PUNPCKLQDQ X2, X2
	MOVQ $0x0a0a0a0a0a0a0a0a, R8
	MOVQ R8, X3
	PUNPCKLQDQ X3, X3
	MOVQ $0x0c0c0c0c0c0c0c0c, R8
	MOVQ R8, X4
	PUNPCKLQDQ X4, X4
	MOVQ $0x0d0d0d0d0d0d0d0d, R8
	MOVQ R8, X5
	PUNPCKLQDQ X5, X5
	MOVQ $0x1b1b1b1b1b1b1b1b, R8
	MOVQ R8, X6
	PUNPCKLQDQ X6, X6

// Bytes >= 0x20 are all valid, including bytes with the high bit set.
vector:
	MOVOU (AX), X0
	MOVOU X0, X7
	PMINUB X1, X7
	PCMPEQB X1, X7
	PMOVMSKB X7, R8
	CMPL R8, $65535
	JE next

// Accept the remaining text controls: TAB, LF, FF, CR, and ESC.
	MOVOU X0, X8
	PCMPEQB X2, X8
	POR X8, X7
	MOVOU X0, X8
	PCMPEQB X3, X8
	POR X8, X7
	MOVOU X0, X8
	PCMPEQB X4, X8
	POR X8, X7
	MOVOU X0, X8
	PCMPEQB X5, X8
	POR X8, X7
	PCMPEQB X6, X0
	POR X0, X7
	PMOVMSKB X7, R8
	CMPL R8, $65535
	JNE invalid

next:
	CMPQ AX, DX
	JE valid
	ADDQ $16, AX
	CMPQ AX, DX
	JBE vector
// Recheck the final 16 bytes without reading beyond the slice.
	MOVQ DX, AX
	JMP vector

scalar:
	TESTQ CX, CX
	JE valid
	MOVBLZX (AX), R8
	CMPB R8, $32
	JAE scalarNext
// Set bits identify forbidden bytes in the range 0x00..0x1f.
	MOVL $0xf7ffc9ff, R9
	BTL R8, R9
	JC invalid

scalarNext:
	INCQ AX
	DECQ CX
	JMP scalar

valid:
	MOVB $1, ret+24(FP)
	RET

invalid:
	MOVB $0, ret+24(FP)
	RET
