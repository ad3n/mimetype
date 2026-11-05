package cdf

import (
	"bytes"
	"encoding/binary"
	"sync"

	"github.com/ad3n/mimetype/internal/scan"
)

const (
	CDFTypeGeneric CDFType = iota
	CDFTypeInstaller
	CDFTypeDoc
	CDFTypePpt
	CDFTypeXls
	CDFTypeMsg
)

const maxPooledBufferCapacity = 64 << 10

const (
	cdfMagic uint64 = 0xE11AB1A1E011CFD0

	dirTypeUserStorage = 1
	dirTypeUserStream  = 2
	dirTypeRootStorage = 5

	dirEntrySize  = 128
	masterSATSize = 109
)

const maxStreamLen = 1 << 16

const (
	propIDNameOfApplication = 0x12

	typeMask        = 0x0fff
	typeVector      = 0x1000
	typeStringASCII = 0x1e
	typeStringWide  = 0x1f

	sectionDeclOffset = 0x1c
)

type (
	CDFType int8

	cdf struct {
		pooledBuffers   [8]*[]byte
		data            []byte
		satSecs         int32s
		ssat            chain
		dir             chain
		pool            chain
		secSize         int
		shortSecSize    int
		satEntries      int
		minStdStream    uint32
		firstSSAT       int32
		rootStreamFirst int32
		rootStreamSize  uint32
		rootChild       int32
		rootStorageUUID [16]byte
		poolBuilt       bool
		pooledCount     uint8
	}

	rootChildIter struct {
		c       *cdf
		d       dirEntry
		n       int
		i       int
		flat    bool
		visited int
		sp      int
		stack   [64]int32
	}

	dirEntry struct {
		name        [32]byte
		nameLen     uint8
		typ         uint8
		left        int32
		right       int32
		child       int32
		streamFirst int32
		size        uint32
		storageUUID [16]byte
	}

	chain struct {
		ids   int32s
		n     int
		first int32
	}

	int32s struct {
		b []byte
	}

	pattern struct {
		needle []byte
		typ    CDFType
	}

	section struct {
		name string
		typ  uint8
		cdf  CDFType
	}
)

var (
	byteBufferPool = sync.Pool{
		New: func() any {
			buf := make([]byte, 0, 512)
			return &buf
		},
	}

	app2type = []pattern{
		{[]byte("WORD"), CDFTypeDoc},
		{[]byte("EXCEL"), CDFTypeXls},
		{[]byte("POWERPOINT"), CDFTypePpt},
		{[]byte("ADVANCED INSTALLER"), CDFTypeInstaller},
		{[]byte("INSTALLSHIELD"), CDFTypeInstaller},
		{[]byte("MICROSOFT PATCH COMPILER"), CDFTypeInstaller},
		{[]byte("NANT"), CDFTypeInstaller},
		{[]byte("WINDOWS INSTALLER"), CDFTypeInstaller},
	}

	name2type = []pattern{
		{[]byte("BOOK"), CDFTypeXls},
		{[]byte("WORKBOOK"), CDFTypeXls},
		{[]byte("WORDDOCUMENT"), CDFTypeDoc},
		{[]byte("POWERPOINT"), CDFTypePpt},
		{[]byte("DIGITALSIGNATURE"), CDFTypeInstaller},
	}

	msiCLSID = []byte{
		0x84, 0x10, 0x0c, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46,
	}

	sectionTypes = []section{
		{"Book", dirTypeUserStream, CDFTypeXls},
		{"Workbook", dirTypeUserStream, CDFTypeXls},
		{"WordDocument", dirTypeUserStream, CDFTypeDoc},
		{"PowerPoint Document", dirTypeUserStream, CDFTypePpt},
		{"__properties_version1.0", dirTypeUserStream, CDFTypeMsg},
		{"__recip_version1.0_#00000000", dirTypeUserStorage, CDFTypeMsg},
	}
)

func Detect(raw []byte) CDFType {
	if len(raw) < 512 {
		return CDFTypeGeneric
	}

	var c cdf
	if !parse(raw, &c) {
		return CDFTypeGeneric
	}

	typ := c.detect()
	if c.pooledCount > 0 {
		c.releaseBuffers()
	}

	return typ
}

func (c *cdf) acquireBuffer() *[]byte {
	if int(c.pooledCount) >= len(c.pooledBuffers) {
		return nil
	}

	buf := byteBufferPool.Get().(*[]byte)
	*buf = (*buf)[:0]
	c.pooledBuffers[c.pooledCount] = buf
	c.pooledCount++

	return buf
}

func (c *cdf) buffer(size int) []byte {
	if size > maxPooledBufferCapacity {
		return make([]byte, size)
	}

	bufp := c.acquireBuffer()
	if bufp == nil {
		return make([]byte, size)
	}

	if cap(*bufp) < size {
		*bufp = make([]byte, size)
	}

	*bufp = (*bufp)[:size]
	return *bufp
}

func (c *cdf) releaseBuffers() {
	for i := range c.pooledCount {
		buf := c.pooledBuffers[i]
		clear(*buf)
		if cap(*buf) <= maxPooledBufferCapacity {
			*buf = (*buf)[:0]
			byteBufferPool.Put(buf)
		}

		c.pooledBuffers[i] = nil
	}

	c.pooledCount = 0
}

func parse(raw []byte, c *cdf) bool {
	if len(raw) < 512 || binary.LittleEndian.Uint64(raw) != cdfMagic {
		return false
	}

	secP2 := binary.LittleEndian.Uint16(raw[30:32])
	shortP2 := binary.LittleEndian.Uint16(raw[32:34])
	if secP2 > 20 || shortP2 > 20 {
		return false
	}

	c.data = raw
	c.secSize = 1 << secP2
	c.shortSecSize = 1 << shortP2
	c.minStdStream = binary.LittleEndian.Uint32(raw[56:60])
	if c.secSize < dirEntrySize {
		return false
	}

	firstDirSec := readSecID(raw[48:52])
	c.firstSSAT = readSecID(raw[60:64])
	firstMSAT := readSecID(raw[68:72])
	nMSAT := binary.LittleEndian.Uint32(raw[72:76])
	masterSAT := int32s{b: raw[76 : 76+4*masterSATSize]}

	c.buildSAT(masterSAT, firstMSAT, nMSAT)
	c.ssat = c.chainOf(c.firstSSAT, len(c.data))
	c.dir = c.chainOf(firstDirSec, len(c.data))

	c.rootStreamFirst = -1
	c.rootChild = -1
	var d dirEntry
	for i, n := 0, c.dirLen(); i < n; i++ {
		if !c.dirAt(i, &d) || d.typ != dirTypeRootStorage {
			continue
		}

		copy(c.rootStorageUUID[:], d.storageUUID[:])
		c.rootChild = d.child

		if d.streamFirst >= 0 {
			c.rootStreamFirst = d.streamFirst
			c.rootStreamSize = d.size
		}

		break
	}

	return true
}

func (c *cdf) rootChildren() rootChildIter {
	it := rootChildIter{c: c, n: c.dirLen()}
	if c.rootChild < 0 || int(c.rootChild) >= it.n {
		it.flat = true
		return it
	}

	it.stack[0] = c.rootChild
	it.sp = 1

	return it
}

func (it *rootChildIter) next() bool {
	if it.flat {
		for it.i < it.n {
			it.i++
			if it.c.dirAt(it.i-1, &it.d) {
				return true
			}
		}

		return false
	}

	for it.sp > 0 {
		it.sp--
		id := it.stack[it.sp]
		if id < 0 || int(id) >= it.n {
			continue
		}

		if it.visited++; it.visited > it.n {
			return false
		}

		if !it.c.dirAt(int(id), &it.d) {
			continue
		}

		for _, sib := range [2]int32{it.d.left, it.d.right} {
			if sib >= 0 && it.sp < len(it.stack) {
				it.stack[it.sp] = sib
				it.sp++
			}
		}

		return true
	}

	return false
}

func (c *cdf) detect() CDFType {
	for _, name := range []string{"\x05SummaryInformation", "\x05DocumentSummaryInformation"} {
		if t, ok := c.detectFromSummary(name); ok {
			return t
		}
	}

	for it := c.rootChildren(); it.next(); {
		if t, ok := lookupSection(it.d.nameBytes(), it.d.typ); ok {
			return t
		}
	}

	return CDFTypeGeneric
}

func (c *cdf) detectFromSummary(streamName string) (CDFType, bool) {
	if bytes.Equal(c.rootStorageUUID[:], msiCLSID) {
		return CDFTypeInstaller, true
	}

	raw, ok := c.userStream(streamName)
	if !ok {
		return CDFTypeGeneric, false
	}

	var appBuf [64]byte
	if app := summaryAppName(raw, appBuf[:]); len(app) > 0 {
		if t, ok := lookupSubstring(app, app2type); ok {
			return t, true
		}
	}

	for it := c.rootChildren(); it.next(); {
		if it.d.nameLen == 0 {
			continue
		}

		if t, ok := lookupSubstring(it.d.nameBytes(), name2type); ok {
			return t, true
		}
	}

	return CDFTypeGeneric, true
}

func (d *dirEntry) nameBytes() []byte { return d.name[:d.nameLen] }

func (ch chain) at(k int) int32 {
	if ch.ids.len() == 0 {
		return ch.first + int32(k) //nolint:gosec // k < n, and n sectors fit in the input
	}

	return ch.ids.at(k)
}

func (c *cdf) chainOf(sid int32, maxBytes int) chain {
	maxN := maxBytes/c.secSize + 1
	ch := chain{first: sid}
	contiguous := true
	for s := sid; s >= 0 && ch.n < maxN; ch.n++ {
		if _, ok := c.sector(s); !ok || int(s) >= c.satLen() {
			break
		}

		next := c.satAt(s)
		if next >= 0 && int64(next) != int64(s)+1 {
			contiguous = false
		}

		s = next
	}

	if contiguous {
		return ch
	}

	buf := c.buffer(4 * ch.n)
	for k, s := 0, sid; k < ch.n; k++ {
		binary.LittleEndian.PutUint32(buf[4*k:], uint32(s)) //nolint:gosec // reinterpret sector id
		s = c.satAt(s)
	}

	ch.ids = int32s{b: buf}
	return ch
}

func (c *cdf) record(ch chain, idx, size int) []byte {
	perSec := c.secSize / size
	if idx < 0 || perSec == 0 {
		return nil
	}

	k, off := idx/perSec, idx%perSec*size
	if k >= ch.n {
		return nil
	}

	buf, ok := c.sector(ch.at(k))
	if !ok || off+size > len(buf) {
		return nil
	}

	return buf[off : off+size]
}

func (c *cdf) ssatAt(i int32) int32 {
	b := c.record(c.ssat, int(i), 4)
	if b == nil {
		return -1
	}

	return readSecID(b)
}

func (c *cdf) shortStreamPool() chain {
	if !c.poolBuilt {
		c.poolBuilt = true
		if c.rootStreamFirst >= 0 {
			c.pool = c.chainOf(c.rootStreamFirst, len(c.data))
		}
	}

	return c.pool
}

func (b int32s) at(i int) int32 {
	return int32(binary.LittleEndian.Uint32(b.b[4*i:])) //nolint:gosec // intentional two's-complement reinterpretation of a sector id
}
func (b int32s) len() int {
	return len(b.b) / 4
}

func readSecID(b []byte) int32 {
	return int32(binary.LittleEndian.Uint32(b)) //nolint:gosec // intentional two's-complement reinterpretation
}

func (c *cdf) satLen() int { return c.satEntries }

func (c *cdf) satAt(i int32) int32 {
	perSec := c.secSize / 4
	secIdx := int(i) / perSec
	entryIdx := int(i) % perSec
	secID := c.satSecs.at(secIdx)
	off := c.secSize*(1+int(secID)) + 4*entryIdx
	return readSecID(c.data[off:])
}

func (c *cdf) sector(secid int32) (_ []byte, ok bool) {
	if secid < 0 {
		return nil, false
	}

	off := int64(c.secSize) * (1 + int64(secid))
	if off >= int64(len(c.data)) {
		return nil, false
	}

	end := min(off+int64(c.secSize), int64(len(c.data)))

	if end-off < 4 {
		return nil, false
	}

	return c.data[off:end], true
}

func (c *cdf) sectorIDs(secid int32) (int32s, bool) {
	buf, ok := c.sector(secid)
	if !ok {
		return int32s{}, ok
	}

	return int32s{b: buf}, true
}

func (c *cdf) buildSAT(masterSAT int32s, firstMSAT int32, nMSAT uint32) {
	if firstMSAT < 0 || nMSAT == 0 {
		c.satSecs = masterSAT
		c.satEntries = c.computeSATLen()
		return
	}

	maxIDs := len(c.data)/c.secSize + 1
	bufp := c.acquireBuffer()
	var buf []byte
	if bufp != nil {
		buf = *bufp
	}

	if cap(buf) < 4*masterSATSize {
		buf = make([]byte, 0, 4*masterSATSize)
	}

	for i := 0; i < masterSAT.len(); i++ {
		if masterSAT.at(i) < 0 {
			break
		}

		buf = append(buf, masterSAT.b[4*i:4*i+4]...)
	}

	perSec := c.secSize/4 - 1
	mid := firstMSAT
chain:
	for j := uint32(0); j < nMSAT && mid >= 0; j++ {
		msa, ok := c.sectorIDs(mid)
		if !ok {
			break
		}

		for k := range perSec {
			if k >= msa.len() || msa.at(k) < 0 {
				break chain
			}

			buf = append(buf, msa.b[4*k:4*k+4]...)
			if len(buf)/4 > maxIDs {
				break chain
			}
		}

		if perSec >= msa.len() {
			break
		}

		mid = msa.at(perSec)
	}

	if bufp != nil {
		*bufp = buf
	}

	c.satSecs = int32s{b: buf}
	c.satEntries = c.computeSATLen()
}

func (c *cdf) computeSATLen() int {
	perSec := c.secSize / 4
	total := 0
	for i := 0; i < c.satSecs.len(); i++ {
		sec := c.satSecs.at(i)
		if sec < 0 {
			break
		}

		off := int64(c.secSize) * (1 + int64(sec))
		if off >= int64(len(c.data)) {
			break
		}

		avail := int64(len(c.data)) - off
		if avail >= int64(c.secSize) {
			total += perSec
			continue
		}

		total += int(avail / 4)
		break
	}

	return total
}

func (c *cdf) readLong(sid int32, limit int) []byte {
	ch := c.chainOf(sid, limit)
	if ch.n == 0 {
		return nil
	}

	if ch.ids.len() == 0 {
		off := int64(c.secSize) * (1 + int64(sid))
		end := min(off+int64(ch.n)*int64(c.secSize), int64(len(c.data)), off+int64(limit))
		return c.data[off:end]
	}

	out := c.buffer(limit)[:0]
	for k := 0; k < ch.n && len(out) < limit; k++ {
		buf, _ := c.sector(ch.at(k))
		out = append(out, buf[:min(len(buf), limit-len(out))]...)
	}

	return out
}

func (c *cdf) readShort(sid int32, limit int) []byte {
	pool := c.shortStreamPool()
	if pool.n == 0 {
		return nil
	}

	out := c.buffer(limit)[:0]
	for sid >= 0 && len(out) < limit {
		end := (int64(sid) + 1) * int64(c.shortSecSize)
		if c.rootStreamSize > 0 && end > int64(c.rootStreamSize) {
			break
		}

		sec := c.record(pool, int(sid), c.shortSecSize)
		if sec == nil {
			break
		}

		out = append(out, sec[:min(len(sec), limit-len(out))]...)
		sid = c.ssatAt(sid)
	}

	return out
}

func (c *cdf) readChain(sid int32, length uint32) []byte {
	limit := min(maxStreamLen, len(c.data))
	if length > 0 && int64(length) < int64(limit) {
		limit = int(length)
	}

	if length < c.minStdStream && c.rootStreamFirst >= 0 {
		return c.readShort(sid, limit)
	}

	return c.readLong(sid, limit)
}

func (c *cdf) dirLen() int { return c.dir.n * (c.secSize / dirEntrySize) }

func (c *cdf) dirAt(i int, out *dirEntry) bool {
	raw := c.record(c.dir, i, dirEntrySize)
	if raw == nil {
		return false
	}

	nameLen := min(int(binary.LittleEndian.Uint16(raw[64:])), 64)
	k := uint8(0)
	for j := 0; j < nameLen/2; j++ {
		lo, hi := raw[2*j], raw[2*j+1]
		if lo == 0 && hi == 0 {
			break
		}

		out.name[k] = lo
		k++
	}

	out.nameLen = k
	out.typ = raw[66]
	out.left = readSecID(raw[68:72])
	out.right = readSecID(raw[72:76])
	out.child = readSecID(raw[76:80])
	out.streamFirst = readSecID(raw[116:120])
	out.size = binary.LittleEndian.Uint32(raw[120:])
	copy(out.storageUUID[:], raw[80:96])
	return true
}

func (c *cdf) userStream(name string) ([]byte, bool) {
	for it := c.rootChildren(); it.next(); {
		if it.d.typ == dirTypeUserStream && string(it.d.nameBytes()) == name {
			buf := c.readChain(it.d.streamFirst, it.d.size)
			if buf == nil {
				return nil, false
			}

			return buf, true
		}
	}

	return nil, false
}

func summaryAppName(stream, dst []byte) []byte {
	if len(stream) < sectionDeclOffset+20 {
		return nil
	}

	sdOff := binary.LittleEndian.Uint32(stream[sectionDeclOffset+16:])
	if uint64(sdOff)+8 > uint64(len(stream)) {
		return nil
	}

	section := stream[sdOff:]
	shLen := binary.LittleEndian.Uint32(section[0:])
	nProps := binary.LittleEndian.Uint32(section[4:])
	if uint64(shLen) > uint64(len(section)) || nProps > 1<<16 || 8+8*nProps > shLen {
		return nil
	}

	for i := range nProps {
		base := 8 + 8*i
		id := binary.LittleEndian.Uint32(section[base:])
		if id != propIDNameOfApplication {
			continue
		}

		off := binary.LittleEndian.Uint32(section[base+4:])
		if uint64(off)+8 > uint64(shLen) {
			return nil
		}

		typ := binary.LittleEndian.Uint32(section[off:])
		if typ&typeVector != 0 {
			return nil
		}

		step := uint32(0)
		switch typ & typeMask {
		case typeStringASCII:
			step = 1
		case typeStringWide:
			step = 2
		default:
			return nil
		}

		slen := binary.LittleEndian.Uint32(section[off+4:])
		start := uint64(off) + 8
		end := start + uint64(slen)*uint64(step)
		if end > uint64(shLen) {
			return nil
		}

		return printableLowBytes(section[start:end], int(step), dst)
	}

	return nil
}

func printableLowBytes(b []byte, step int, dst []byte) []byte {
	out := dst[:0]
	if cap(out) < len(b)/step {
		out = make([]byte, 0, len(b)/step)
	}

	for i := 0; i+step <= len(b); i += step {
		c := b[i]
		if c == 0 {
			break
		}

		if c >= 0x20 && c < 0x7f {
			out = append(out, c)
		}
	}

	return out
}

func lookupSubstring(v []byte, t []pattern) (CDFType, bool) {
	s := scan.Bytes(v)
	for _, p := range t {
		if i, _ := s.Search(p.needle, scan.IgnoreCase); i != -1 {
			return p.typ, true
		}
	}

	return CDFTypeGeneric, false
}

func lookupSection(name []byte, typ uint8) (CDFType, bool) {
	for _, s := range sectionTypes {
		if s.typ == typ && string(name) == s.name {
			return s.cdf, true
		}
	}

	return CDFTypeGeneric, false
}
