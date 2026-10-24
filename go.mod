module github.com/ad3n/mimetype

go 1.27

retract (
	// v1.4.15 spends quadratic time for specially crafted CDF files.
	// GHSA-jjmc-jx7j-hp68
	v1.4.15
	// v1.4.14 had an int overflow causing slice index panic on 32bit arch. #829
	v1.4.14
	// v1.4.4 had a test file detected as malicious by antivirus software. #575
	v1.4.4
)
