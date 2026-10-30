package handler

// 极简 XLSX 读写（仅依赖标准库）
//
// 为什么自己实现：本项目为离线构建（无外网），go.mod 里没有 excelize，
// 也不希望为此引入第三方依赖；导入导出只需要「一张表 + 表头」这种简单结构，
// 因此用 archive/zip + encoding/xml 直接读写 OOXML。
//
// 写出：多工作表、表头加粗填色、首行冻结、全部单元格按文本写出（防止 Excel 把
// 身份证/日期/编号自动转成数值或日期格式），单元格使用 inlineStr。
//
// 读入：兼容 Excel / WPS / LibreOffice / 本程序导出的文件，支持共享字符串、
// 内联字符串、富文本、数字、公式结果、日期序列号（按单元格样式里的 numFmtId
// 判断是否为日期格式再转成 YYYY-MM-DD）。

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

// xlsxSheet 一个工作表（第一行为表头）
type xlsxSheet struct {
	Name   string
	Header []string
	Rows   [][]string
	Widths []float64 // 可选：每列宽度（字符数）；缺省自动估算
}

func xesc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// xlsxColName 1 -> A, 27 -> AA
func xlsxColName(n int) string {
	if n <= 0 {
		return "A"
	}
	var s string
	for n > 0 {
		n--
		s = string(rune('A'+n%26)) + s
		n /= 26
	}
	return s
}

// xlsxColIndex A -> 1, AA -> 27（非法返回 0）
func xlsxColIndex(s string) int {
	n := 0
	for _, r := range strings.ToUpper(strings.TrimSpace(s)) {
		if r < 'A' || r > 'Z' {
			break
		}
		n = n*26 + int(r-'A'+1)
	}
	return n
}

// xlsxTextWidth 估算显示宽度（中日韩字符按 2 个字符宽）
func xlsxTextWidth(s string) float64 {
	w := 0.0
	for _, r := range s {
		if r == '\n' || r == '\r' {
			continue
		}
		if r > 127 {
			w += 2
		} else {
			w++
		}
	}
	return w
}

// sanitizeSheetName 工作表名限制：<=31 字符，不能含 []:*?/\
func sanitizeSheetName(name string, idx int) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Sheet" + strconv.Itoa(idx+1)
	}
	repl := func(r rune) rune {
		switch r {
		case '[', ']', ':', '*', '?', '/', '\\':
			return '_'
		}
		return r
	}
	name = strings.Map(repl, name)
	rs := []rune(name)
	if len(rs) > 31 {
		rs = rs[:31]
	}
	return string(rs)
}

// writeXLSX 把若干工作表写成 xlsx 字节流
func writeXLSX(w io.Writer, sheets []xlsxSheet) error {
	if len(sheets) == 0 {
		return fmt.Errorf("没有可写出的工作表")
	}
	zw := zip.NewWriter(w)
	defer zw.Close()

	writeFile := func(name string, body []byte) error {
		f, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = f.Write(body)
		return err
	}

	// [Content_Types].xml
	var ct strings.Builder
	ct.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	ct.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	ct.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`)
	ct.WriteString(`<Default Extension="xml" ContentType="application/xml"/>`)
	ct.WriteString(`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>`)
	ct.WriteString(`<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`)
	for i := range sheets {
		ct.WriteString(fmt.Sprintf(`<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, i+1))
	}
	ct.WriteString(`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>`)
	ct.WriteString(`<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>`)
	ct.WriteString(`</Types>`)
	if err := writeFile("[Content_Types].xml", []byte(ct.String())); err != nil {
		return err
	}

	// _rels/.rels
	rels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>` +
		`</Relationships>`
	if err := writeFile("_rels/.rels", []byte(rels)); err != nil {
		return err
	}

	// docProps
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	core := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" ` +
		`xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" ` +
		`xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` +
		`<dc:title>族谱成员档案</dc:title><dc:creator>族谱系统</dc:creator>` +
		`<dcterms:created xsi:type="dcterms:W3CDTF">` + now + `</dcterms:created>` +
		`<dcterms:modified xsi:type="dcterms:W3CDTF">` + now + `</dcterms:modified>` +
		`</cp:coreProperties>`
	if err := writeFile("docProps/core.xml", []byte(core)); err != nil {
		return err
	}
	app := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" ` +
		`xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">` +
		`<Application>族谱系统</Application></Properties>`
	if err := writeFile("docProps/app.xml", []byte(app)); err != nil {
		return err
	}

	// xl/workbook.xml
	var wb strings.Builder
	wb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	wb.WriteString(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" `)
	wb.WriteString(`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	wb.WriteString(`<sheets>`)
	for i, s := range sheets {
		wb.WriteString(fmt.Sprintf(`<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, xesc(sanitizeSheetName(s.Name, i)), i+1, i+1))
	}
	wb.WriteString(`</sheets></workbook>`)
	if err := writeFile("xl/workbook.xml", []byte(wb.String())); err != nil {
		return err
	}

	// xl/_rels/workbook.xml.rels
	var wbr strings.Builder
	wbr.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	wbr.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i := range sheets {
		wbr.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, i+1, i+1))
	}
	wbr.WriteString(`<Relationship Id="rIdStyles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`)
	wbr.WriteString(`</Relationships>`)
	if err := writeFile("xl/_rels/workbook.xml.rels", []byte(wbr.String())); err != nil {
		return err
	}

	// xl/styles.xml
	// cellXfs: 0=普通；1=表头（加粗、深色底、白字、居中、自动换行）；2=文本格式
	styles := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<fonts count="2">` +
		`<font><sz val="11"/><color rgb="FF3A3226"/><name val="宋体"/><charset val="134"/></font>` +
		`<font><b/><sz val="11"/><color rgb="FFFFFFFF"/><name val="宋体"/><charset val="134"/></font>` +
		`</fonts>` +
		`<fills count="3">` +
		`<fill><patternFill patternType="none"/></fill>` +
		`<fill><patternFill patternType="gray125"/></fill>` +
		`<fill><patternFill patternType="solid"><fgColor rgb="FF8C5A3C"/><bgColor indexed="64"/></patternFill></fill>` +
		`</fills>` +
		`<borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>` +
		`<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>` +
		`<cellXfs count="3">` +
		`<xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>` +
		`<xf numFmtId="0" fontId="1" fillId="2" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="center" vertical="center" wrapText="1"/></xf>` +
		`<xf numFmtId="49" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>` +
		`</cellXfs>` +
		`<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>` +
		`<dxfs count="0"/>` +
		`<tableStyles count="0" defaultTableStyle="TableStyleMedium2" defaultPivotStyle="PivotStyleLight16"/>` +
		`</styleSheet>`
	if err := writeFile("xl/styles.xml", []byte(styles)); err != nil {
		return err
	}

	// xl/worksheets/sheetN.xml
	for si, s := range sheets {
		var b strings.Builder
		b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
		b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" `)
		b.WriteString(`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
		b.WriteString(`<sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>`)
		b.WriteString(`<sheetFormatPr defaultRowHeight="16"/>`)

		// 列宽
		ncol := len(s.Header)
		if ncol == 0 {
			for _, r := range s.Rows {
				if len(r) > ncol {
					ncol = len(r)
				}
			}
		}
		if ncol > 0 {
			b.WriteString(`<cols>`)
			for c := 0; c < ncol; c++ {
				width := 0.0
				if c < len(s.Widths) && s.Widths[c] > 0 {
					width = s.Widths[c]
				} else {
					if c < len(s.Header) {
						width = xlsxTextWidth(s.Header[c])
					}
					for _, r := range s.Rows {
						if c < len(r) {
							if w := xlsxTextWidth(r[c]); w > width {
								width = w
							}
						}
					}
					width += 2
					if width < 8 {
						width = 8
					}
					if width > 60 {
						width = 60
					}
				}
				b.WriteString(fmt.Sprintf(`<col min="%d" max="%d" width="%.1f" customWidth="1"/>`, c+1, c+1, width))
			}
			b.WriteString(`</cols>`)
		}

		b.WriteString(`<sheetData>`)
		rowNo := 0
		emitRow := func(cells []string, style int, header bool) {
			rowNo++
			if header {
				b.WriteString(fmt.Sprintf(`<row r="%d" ht="26" customHeight="1">`, rowNo))
			} else {
				b.WriteString(fmt.Sprintf(`<row r="%d">`, rowNo))
			}
			for ci, v := range cells {
				if v == "" && !header {
					continue
				}
				ref := xlsxColName(ci+1) + strconv.Itoa(rowNo)
				b.WriteString(fmt.Sprintf(`<c r="%s" s="%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, style, xesc(v)))
			}
			b.WriteString(`</row>`)
		}
		emitRow(s.Header, 1, true)
		for _, r := range s.Rows {
			emitRow(r, 2, false)
		}
		b.WriteString(`</sheetData>`)
		b.WriteString(`<pageMargins left="0.7" right="0.7" top="0.75" bottom="0.75" header="0.3" footer="0.3"/>`)
		b.WriteString(`</worksheet>`)
		if err := writeFile(fmt.Sprintf("xl/worksheets/sheet%d.xml", si+1), []byte(b.String())); err != nil {
			return err
		}
	}

	return zw.Close()
}

// ---------------------------------------------------------------------------
// 读入
// ---------------------------------------------------------------------------

type xlsxCellXML struct {
	R  string `xml:"r,attr"`
	T  string `xml:"t,attr"`
	S  int    `xml:"s,attr"`
	V  string `xml:"v"`
	IS struct {
		T string `xml:"t"`
		R []struct {
			T string `xml:"t"`
		} `xml:"r"`
	} `xml:"is"`
}

type xlsxRowXML struct {
	R int           `xml:"r,attr"`
	C []xlsxCellXML `xml:"c"`
}

// readXLSX 读取工作簿第一个工作表，返回按行号排列的二维字符串表（含表头）。
// 行号即返回切片下标+1；中间空行会保留为空切片，便于报错时定位 Excel 行号。
func readXLSX(data []byte) ([][]string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("不是有效的 xlsx 文件：%w", err)
	}
	files := map[string][]byte{}
	for _, f := range zr.File {
		name := strings.TrimPrefix(f.Name, "/")
		if f.FileInfo().IsDir() {
			continue
		}
		if f.UncompressedSize64 > 128<<20 {
			return nil, fmt.Errorf("xlsx 内部文件过大：%s", name)
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}
		files[name] = b
	}

	get := func(names ...string) []byte {
		for _, n := range names {
			if b, ok := files[n]; ok {
				return b
			}
		}
		return nil
	}

	shared := parseSharedStrings(get("xl/sharedStrings.xml"))
	dateStyles := parseDateStyles(get("xl/styles.xml"))

	sheetPath := resolveFirstSheetPath(get("xl/workbook.xml"), get("xl/_rels/workbook.xml.rels"))
	sheetXML := get(sheetPath, "xl/worksheets/sheet1.xml")
	if len(sheetXML) == 0 {
		return nil, fmt.Errorf("xlsx 中找不到工作表数据")
	}

	dec := xml.NewDecoder(bytes.NewReader(sheetXML))
	var rows [][]string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("解析工作表失败：%w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "row" {
			continue
		}
		var rx xlsxRowXML
		if err := dec.DecodeElement(&rx, &se); err != nil {
			return nil, fmt.Errorf("解析工作表行失败：%w", err)
		}
		rowNo := rx.R
		if rowNo <= 0 {
			rowNo = len(rows) + 1
		}
		if rowNo > 200000 {
			return nil, fmt.Errorf("行数超过上限（>200000）")
		}
		cells := make([]string, 0, len(rx.C))
		lastCol := len(cells)
		for ci, c := range rx.C {
			col := xlsxColIndex(c.R)
			if col <= 0 {
				col = ci + 1
			}
			if col > 512 {
				continue
			}
			for len(cells) < col {
				cells = append(cells, "")
			}
			cells[col-1] = xlsxCellValue(c, shared, dateStyles)
			if col > lastCol {
				lastCol = col
			}
		}
		if len(cells) > lastCol {
			cells = cells[:lastCol]
		}
		for len(rows) < rowNo-1 {
			rows = append(rows, nil)
		}
		rows = append(rows, cells)
	}
	return rows, nil
}

func xlsxCellValue(c xlsxCellXML, shared []string, dateStyles []bool) string {
	// 内联字符串（本程序导出即此形式）
	if c.T == "inlineStr" {
		if c.IS.T != "" {
			return c.IS.T
		}
		var sb strings.Builder
		for _, r := range c.IS.R {
			sb.WriteString(r.T)
		}
		return sb.String()
	}
	raw := strings.TrimSpace(c.V)
	switch c.T {
	case "s":
		idx, err := strconv.Atoi(raw)
		if err != nil || idx < 0 || idx >= len(shared) {
			return ""
		}
		return shared[idx]
	case "str", "e", "d":
		return raw
	case "b":
		if raw == "1" {
			return "是"
		}
		return "否"
	}
	// 数字：若是日期样式，转成 YYYY-MM-DD；否则去掉浮点尾巴
	if raw == "" {
		return ""
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		if c.S >= 0 && c.S < len(dateStyles) && dateStyles[c.S] {
			if d := excelSerialToDate(f); d != "" {
				return d
			}
		}
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return raw
}

func excelSerialToDate(f float64) string {
	if f <= 0 || f > 2958465 { // 9999-12-31
		return ""
	}
	days := int(math.Floor(f))
	t := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).AddDate(0, 0, days)
	if t.Year() < 1900 || t.Year() > 9999 {
		return ""
	}
	return t.Format("2006-01-02")
}

// parseSharedStrings 解析共享字符串表（支持富文本）
func parseSharedStrings(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	dec := xml.NewDecoder(bytes.NewReader(b))
	var out []string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "si" {
			continue
		}
		var sb strings.Builder
		depth := 1
		for depth > 0 {
			t2, err := dec.Token()
			if err != nil {
				break
			}
			switch e := t2.(type) {
			case xml.StartElement:
				depth++
				if e.Name.Local == "t" {
					var s string
					if dec.DecodeElement(&s, &e) == nil {
						sb.WriteString(s)
					}
					depth--
				}
			case xml.EndElement:
				depth--
			}
		}
		out = append(out, sb.String())
	}
	return out
}

// parseDateStyles 返回「cellXfs 下标 -> 是否日期格式」
func parseDateStyles(b []byte) []bool {
	if len(b) == 0 {
		return nil
	}
	dec := xml.NewDecoder(bytes.NewReader(b))
	custom := map[int]bool{}
	var styles []bool
	inCellXfs := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch e := tok.(type) {
		case xml.StartElement:
			switch e.Name.Local {
			case "numFmt":
				id, code := 0, ""
				for _, a := range e.Attr {
					switch a.Name.Local {
					case "numFmtId":
						id, _ = strconv.Atoi(a.Value)
					case "formatCode":
						code = a.Value
					}
				}
				custom[id] = isDateNumFmt(code)
			case "cellXfs":
				inCellXfs = true
			case "xf":
				if inCellXfs {
					id := 0
					for _, a := range e.Attr {
						if a.Name.Local == "numFmtId" {
							id, _ = strconv.Atoi(a.Value)
						}
					}
					styles = append(styles, isBuiltinDateFmt(id) || custom[id])
				}
			}
		case xml.EndElement:
			if e.Name.Local == "cellXfs" {
				inCellXfs = false
			}
		}
	}
	return styles
}

func isBuiltinDateFmt(id int) bool {
	switch {
	case id >= 14 && id <= 22:
		return true
	case id >= 27 && id <= 36: // 中日韩本地化日期
		return true
	case id >= 45 && id <= 47:
		return true
	case id >= 50 && id <= 58: // 中日韩本地化日期
		return true
	}
	return false
}

func isDateNumFmt(code string) bool {
	// 去掉引号内的字面量、[颜色/条件]、转义字符后判断
	var sb strings.Builder
	inQuote := false
	inBracket := false
	for i := 0; i < len(code); i++ {
		c := code[i]
		switch {
		case c == '"':
			inQuote = !inQuote
		case inQuote:
			continue
		case c == '[':
			inBracket = true
		case c == ']':
			inBracket = false
		case inBracket:
			continue
		case c == '\\' || c == '_' || c == '*':
			i++
		default:
			sb.WriteByte(c)
		}
	}
	s := strings.ToLower(sb.String())
	// 含 y（年）或 d（日）即视为日期；纯时间格式（hh:mm:ss）不含这两个字母
	return strings.ContainsAny(s, "yd")
}

// resolveFirstSheetPath 从 workbook.xml + rels 解析第一个工作表的实际路径
func resolveFirstSheetPath(wb, rels []byte) string {
	const relNS = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	rid := ""
	target := ""
	if len(wb) > 0 {
		dec := xml.NewDecoder(bytes.NewReader(wb))
		for {
			tok, err := dec.Token()
			if err == io.EOF || err != nil {
				break
			}
			se, ok := tok.(xml.StartElement)
			if !ok || se.Name.Local != "sheet" {
				continue
			}
			for _, a := range se.Attr {
				switch a.Name.Local {
				case "id":
					if a.Name.Space == relNS || rid == "" {
						rid = a.Value
					}
				}
			}
			if rid != "" {
				break
			}
		}
	}
	if len(rels) > 0 && rid != "" {
		dec := xml.NewDecoder(bytes.NewReader(rels))
		for {
			tok, err := dec.Token()
			if err == io.EOF || err != nil {
				break
			}
			se, ok := tok.(xml.StartElement)
			if !ok || se.Name.Local != "Relationship" {
				continue
			}
			id, tgt := "", ""
			for _, a := range se.Attr {
				switch a.Name.Local {
				case "Id":
					id = a.Value
				case "Target":
					tgt = a.Value
				}
			}
			if id == rid && tgt != "" {
				target = tgt
				break
			}
		}
	}
	if target == "" {
		return "xl/worksheets/sheet1.xml"
	}
	target = strings.TrimPrefix(target, "/")
	if strings.HasPrefix(target, "xl/") {
		return target
	}
	return "xl/" + target
}
