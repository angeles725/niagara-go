package hdbread

import "encoding/binary"

// Record-store layout of variable-length histories (LogRecord, AuditRecord: isFixedSize() false,
// BLogRecord.java:128-131, BAuditRecord.java:127-130). Sources are under
// organized/history/history-rt/vineflower/com/tridium/history/file/recstore/ (N4, file:line).
// Offsets are relative to the data section, which starts at the .hdb dataOffset
// (BFileHistoryTable.java:759-761 hands the store a RandomAccess shifted by dataOffset).
//
//	[0:256]   header, 7 big-endian ints then zero padding (RecordStoreHeader.java:9-10,98-114):
//	          magic 0x0ADDAB01 (182299393), version (written 1, read and ignored :89),
//	          blockSize, pageBlocks, firstPage, lastPage, recordCount (-1 pages: no page yet)
//	page i    at 256 + i*pageSize, pageSize = pageBlocks*blockSize + 12 (RecordStore.java:566,599;
//	          Page.java:57-59); pageCount = (section length - 256) / pageSize (RecordStore.java:78-81)
//	page      recordCount, first, free: three ints (Page.java:209-214), then pageBlocks blocks;
//	          block b at 12 + b*blockSize (WriteBlock.java:17-22, ReadBlock.java:20-30)
//	block     blocksInRec, recBlock: two ints (Block.HEADER_SIZE 8), then blockSize-8 payload bytes.
//	          A record of n bytes takes ceil(n / (blockSize-8)) consecutive blocks of one page,
//	          recBlock 0, 1, ... (Page.java:93-113, RecordStore.java:604-613 never splits a record
//	          across pages); its bytes are the payloads in order, the last one zero padded.
//	order     RecordStoreCursor.java:27-45: start at firstPage, block page.first
//	          (RecordStore.java:294-299); after a record take the next block, and when it is past
//	          the page or its blocksInRec is 0 move to the next page at block 0; the next page of
//	          i is i+1, wrapping to 0 at pageCount, none after lastPage (Page.java:31-42).
//	          The store wraps by clearing the oldest page (RecordStore.java:628-676), so firstPage
//	          can be after lastPage; only the first page is trimmed record by record
//	          (Page.java:146-170), which moves its first block forward.
//	record    BHistoryRecord.read (javax/baja/history/BHistoryRecord.java:96-100): BAbsTime.decode
//	          (baja/javax/baja/sys/BAbsTime.java:635-650; decode48 is decode, :660-662) for both
//	          file versions, so a version-1 8-byte long and a version-2 6-byte encode48 timestamp
//	          both read; then doRead:
//	          LogRecord   readUTF logName, readInt severity, readUTF message, readUTF exception
//	                      (com/tridium/history/log/BLogRecord.java:134-139)
//	          AuditRecord readUTF operation, target, slotName, oldValue, value, userName
//	                      (com/tridium/history/audit/BAuditRecord.java:133-140)
//	readUTF   a 2-byte length, then Java modified UTF-8 of UTF-16 code units (ReadBlock.java:189-246;
//	          written by javax/baja/nre/util/ByteBuffer.writeUTF, docSource nre ByteBuffer.java:486)
//
// A reader that ends a record before its last declared block would make the N4 cursor resume
// inside the record, and one that needs more bytes hits EOFException (ReadBlock.java:61-66); both
// are refused here as format errors instead of being guessed around.
const (
	recstoreMagic      = 0x0addab01 // RecordStoreHeader.java:9
	recstoreHeaderSize = 256        // RecordStoreHeader.java:10
	pageHeaderSize     = 12         // Page.java:10
	blockHeaderSize    = 8          // Block.java:4
	// storeFieldsSize is the 7 header ints readStoreHeader reads, magic through recordCount
	// (RecordStoreHeader.java:98-114); the rest of the 256-byte header is padding.
	storeFieldsSize = 7 * 4
)

// storeHeader is the record-store header and the page geometry derived from it.
type storeHeader struct {
	blockSize, pageBlocks int
	firstPage, lastPage   int
	recordCount           int
	pageSize, pageCount   int
}

// storedRecord is one record's bytes: the payloads of its blocks, where it starts.
type storedRecord struct {
	page, block, blocks int
	data                []byte
}

func be32(b []byte, i int) int { return int(int32(binary.BigEndian.Uint32(b[i:]))) }

// readStoreHeader checks the header and the geometry; the pages must fit in the section.
func readStoreHeader(sect []byte) (storeHeader, error) {
	var h storeHeader
	if len(sect) < storeFieldsSize {
		return h, formatErr("data section of %d bytes has no record-store header", len(sect))
	}
	if m := binary.BigEndian.Uint32(sect); m != recstoreMagic {
		return h, formatErr("bad record-store magic %08x (RecordStoreHeader.java reads 0addab01)", m)
	}
	h.blockSize, h.pageBlocks = be32(sect, 8), be32(sect, 12)
	h.firstPage, h.lastPage, h.recordCount = be32(sect, 16), be32(sect, 20), be32(sect, 24)
	if h.blockSize <= blockHeaderSize || h.pageBlocks <= 0 || int64(h.blockSize)*int64(h.pageBlocks) > int64(len(sect)) {
		return h, formatErr("block size %d x %d page blocks does not fit a %d-byte data section",
			h.blockSize, h.pageBlocks, len(sect))
	}
	h.pageSize = h.pageBlocks*h.blockSize + pageHeaderSize
	if len(sect) > recstoreHeaderSize {
		h.pageCount = (len(sect) - recstoreHeaderSize) / h.pageSize
	}
	return h, nil
}

// walkStore visits the records of a record store in RecordStoreCursor order, each before the
// next block is looked at (so a record that runs past its blocks fails as itself).
func walkStore(sect []byte, visit func(storeHeader, storedRecord) error) (storeHeader, error) {
	h, err := readStoreHeader(sect)
	if err != nil || h.recordCount == 0 || h.firstPage == -1 { // RecordStore.scan: an empty cursor
		return h, err
	}
	for _, p := range []int{h.firstPage, h.lastPage} {
		if p < 0 || p >= h.pageCount {
			return h, formatErr("firstPage %d / lastPage %d outside the %d pages", h.firstPage,
				h.lastPage, h.pageCount)
		}
	}
	page := h.firstPage
	block := be32(sect, recstoreHeaderSize+page*h.pageSize+4) // the first page's first block
	if block < 0 || block >= h.pageBlocks {
		return h, formatErr("first page %d starts at block %d, a page has %d", page, block, h.pageBlocks)
	}
	for visited := 0; visited < h.pageCount; visited++ { // bounded: a corrupt ring cannot loop
		base := recstoreHeaderSize + page*h.pageSize + pageHeaderSize
		for block < h.pageBlocks {
			r, err := blockRecord(sect, h, base, page, block)
			if err != nil {
				return h, err
			}
			if r.blocks == 0 {
				break
			}
			if err := visit(h, r); err != nil {
				return h, err
			}
			block += r.blocks
		}
		if page == h.lastPage {
			break
		}
		if page, block = page+1, 0; page == h.pageCount {
			page = 0
		}
	}
	return h, nil
}

// blockRecord reads the record that starts at block of page (blocks 0: no record there).
func blockRecord(sect []byte, h storeHeader, base, page, block int) (storedRecord, error) {
	at := base + block*h.blockSize
	n, rb := be32(sect, at), be32(sect, at+4)
	r := storedRecord{page: page, block: block, blocks: n}
	if n == 0 {
		return r, nil
	}
	if rb != 0 {
		return r, formatErr("page %d block %d continues a record (recBlock %d), no record starts there",
			page, block, rb)
	}
	if n < 0 || n > h.pageBlocks-block {
		return r, formatErr("page %d block %d declares %d blocks, the page has %d from there", page, block,
			n, h.pageBlocks-block)
	}
	for k := 0; k < n; k++ {
		b := at + k*h.blockSize
		if bn, bk := be32(sect, b), be32(sect, b+4); bn != n || bk != k {
			return r, formatErr("page %d block %d is part %d/%d of a record, expected part %d/%d", page,
				block+k, bk, bn, k, n)
		}
		r.data = append(r.data, sect[b+blockHeaderSize:b+h.blockSize]...)
	}
	return r, nil
}
