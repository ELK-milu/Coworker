import React, { useState, useEffect, useRef, useCallback, Suspense } from 'react';
import { Spin, Typography, Button, Toast } from '@douyinfe/semi-ui';
import { IconSave } from '@douyinfe/semi-icons';
import { saveFileContent } from '../services/api';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

const { Text } = Typography;

const TEXT_EXTS = new Set([
  'txt','log','conf','cfg','ini','env','gitignore','editorconfig','makefile','dockerfile',
]);
const CODE_EXTS = new Set([
  'js','jsx','ts','tsx','py','go','java','c','cpp','h','rs','rb','php',
  'sh','bash','css','scss','sass','less','html','htm','json','xml','yaml','yml',
  'toml','sql','vue','svelte','swift','kt','r','lua','pl','md','csv',
]);
const IMG_EXTS = new Set(['png','jpg','jpeg','gif','bmp','svg','webp','ico']);
const VIDEO_EXTS = new Set(['mp4','webm','mov','avi','mkv']);
const AUDIO_EXTS = new Set(['mp3','wav','ogg','flac','m4a']);

// 通用保存函数
async function doSave(userId, filePath, fileName, blob) {
  await saveFileContent(userId, filePath, blob, fileName);
  Toast.success('已保存');
}

// 通用工具栏
function Toolbar({ onSave, saving, extra }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '6px 8px', borderBottom: '1px solid var(--semi-color-border)', flexShrink: 0 }}>
      <Button size="small" icon={<IconSave />} loading={saving} onClick={onSave} type="primary" theme="solid">保存</Button>
      {extra}
    </div>
  );
}

function getExt(name) {
  return (name || '').split('.').pop()?.toLowerCase() || '';
}

// 富文本格式按钮
const FMT_BTNS = [
  { cmd: 'bold', label: 'B', style: { fontWeight: 'bold' } },
  { cmd: 'italic', label: 'I', style: { fontStyle: 'italic' } },
  { cmd: 'underline', label: 'U', style: { textDecoration: 'underline' } },
  { cmd: 'strikeThrough', label: 'S', style: { textDecoration: 'line-through' } },
];
const ALIGN_BTNS = [
  { cmd: 'justifyLeft', label: '⫷' },
  { cmd: 'justifyCenter', label: '⫿' },
  { cmd: 'justifyRight', label: '⫸' },
];

// DOCX 编辑器 (mammoth → HTML → contentEditable → docx 库保存)
function DocxEditor({ blob, userId, filePath, fileName }) {
  const editorRef = useRef(null);
  const [html, setHtml] = useState(null);
  const [err, setErr] = useState(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!blob) return;
    let cancelled = false;
    (async () => {
      try {
        const mammoth = await import('mammoth');
        const buf = await blob.arrayBuffer();
        const result = await mammoth.convertToHtml({ arrayBuffer: buf });
        if (!cancelled) setHtml(result.value);
      } catch (e) {
        if (!cancelled) setErr(e.message);
      }
    })();
    return () => { cancelled = true; };
  }, [blob]);

  const handleSave = useCallback(async () => {
    if (!editorRef.current || !userId) return;
    setSaving(true);
    try {
      const { Document, Packer, Paragraph, TextRun, AlignmentType } = await import('docx');
      const paragraphs = [];
      const el = editorRef.current;
      // 遍历顶层节点，转换为 docx 段落
      for (const node of el.childNodes) {
        if (node.nodeType === Node.TEXT_NODE) {
          if (node.textContent.trim()) paragraphs.push(new Paragraph({ children: [new TextRun(node.textContent)] }));
          continue;
        }
        const tag = (node.tagName || '').toLowerCase();
        const runs = [];
        // 递归提取文本和格式
        const walk = (n, fmt = {}) => {
          if (n.nodeType === Node.TEXT_NODE) {
            if (n.textContent) runs.push(new TextRun({ text: n.textContent, bold: fmt.b, italics: fmt.i, underline: fmt.u ? {} : undefined, strike: fmt.s }));
            return;
          }
          const t = (n.tagName || '').toLowerCase();
          const f = { ...fmt };
          if (t === 'b' || t === 'strong') f.b = true;
          if (t === 'i' || t === 'em') f.i = true;
          if (t === 'u') f.u = true;
          if (t === 's' || t === 'del') f.s = true;
          for (const c of n.childNodes) walk(c, f);
        };
        walk(node);
        const align = node.style?.textAlign;
        const alignment = align === 'center' ? AlignmentType.CENTER : align === 'right' ? AlignmentType.RIGHT : undefined;
        paragraphs.push(new Paragraph({ children: runs, alignment }));
      }
      const doc = new Document({ sections: [{ children: paragraphs }] });
      const buf = await Packer.toBlob(doc);
      await doSave(userId, filePath, fileName, buf);
    } catch (e) {
      Toast.error('保存失败: ' + e.message);
    } finally {
      setSaving(false);
    }
  }, [userId, filePath, fileName]);

  const execCmd = (cmd) => document.execCommand(cmd, false, null);

  if (err) return <Text type="danger">DOCX 渲染失败: {err}</Text>;
  if (html === null) return <div style={{ display: 'flex', justifyContent: 'center', paddingTop: 40 }}><Spin /></div>;

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      <Toolbar onSave={handleSave} saving={saving} extra={
        <>
          <div style={{ width: 1, height: 20, background: 'var(--semi-color-border)', margin: '0 4px' }} />
          {FMT_BTNS.map(b => (
            <button key={b.cmd} onClick={() => execCmd(b.cmd)} title={b.cmd}
              style={{ ...b.style, border: '1px solid var(--semi-color-border)', borderRadius: 3, width: 28, height: 28, cursor: 'pointer', background: 'transparent', fontSize: 13 }}>
              {b.label}
            </button>
          ))}
          <div style={{ width: 1, height: 20, background: 'var(--semi-color-border)', margin: '0 2px' }} />
          {ALIGN_BTNS.map(b => (
            <button key={b.cmd} onClick={() => execCmd(b.cmd)} title={b.cmd}
              style={{ border: '1px solid var(--semi-color-border)', borderRadius: 3, width: 28, height: 28, cursor: 'pointer', background: 'transparent', fontSize: 13 }}>
              {b.label}
            </button>
          ))}
          <div style={{ width: 1, height: 20, background: 'var(--semi-color-border)', margin: '0 2px' }} />
          <button onClick={() => execCmd('insertUnorderedList')} title="无序列表"
            style={{ border: '1px solid var(--semi-color-border)', borderRadius: 3, width: 28, height: 28, cursor: 'pointer', background: 'transparent', fontSize: 13 }}>•</button>
          <button onClick={() => execCmd('insertOrderedList')} title="有序列表"
            style={{ border: '1px solid var(--semi-color-border)', borderRadius: 3, width: 28, height: 28, cursor: 'pointer', background: 'transparent', fontSize: 13 }}>1.</button>
        </>
      } />
      <div
        ref={editorRef}
        contentEditable
        suppressContentEditableWarning
        dangerouslySetInnerHTML={{ __html: html }}
        style={{
          flex: 1, overflow: 'auto', padding: '20px 24px',
          fontFamily: '"Segoe UI", "Microsoft YaHei", sans-serif',
          fontSize: 14, lineHeight: 1.8, outline: 'none',
          background: '#fff', minHeight: 0,
        }}
      />
    </div>
  );
}

// XLSX 编辑器 (Fortune-sheet + ExcelJS 保存)
function XlsxEditor({ blob, userId, filePath, fileName }) {
  const [content, setContent] = useState(null);
  const [err, setErr] = useState(null);
  const [saving, setSaving] = useState(false);
  const sheetsRef = useRef(null);

  useEffect(() => {
    if (!blob) return;
    let cancelled = false;
    (async () => {
      try {
        const [{ Workbook }, LuckyExcelMod] = await Promise.all([
          import('@fortune-sheet/react'),
          import('luckyexcel'),
          import('@fortune-sheet/react/dist/index.css'),
        ]);
        const LuckyExcel = LuckyExcelMod.default || LuckyExcelMod;
        const file = new File([blob], 'file.xlsx', { type: blob.type });
        LuckyExcel.transformExcelToLucky(file, (exportJson) => {
          if (cancelled) return;
          if (!exportJson?.sheets?.length) { setErr('无法解析 Excel 文件'); return; }
          sheetsRef.current = exportJson.sheets;
          setContent({ Workbook, sheets: exportJson.sheets });
        }, (errMsg) => {
          if (!cancelled) setErr(typeof errMsg === 'string' ? errMsg : '解析失败');
        });
      } catch (e) {
        if (!cancelled) setErr(e.message);
      }
    })();
    return () => { cancelled = true; };
  }, [blob]);

  const handleSave = useCallback(async () => {
    if (!sheetsRef.current || !userId) return;
    setSaving(true);
    try {
      const ExcelJS = (await import('exceljs')).default;
      const wb = new ExcelJS.Workbook();
      for (const s of sheetsRef.current) {
        const ws = wb.addWorksheet(s.name || 'Sheet');
        const cells = s.celldata || [];
        for (const c of cells) {
          const val = c.v?.v ?? c.v?.m ?? '';
          if (val !== '' && val != null) {
            const cell = ws.getCell(c.r + 1, c.c + 1);
            cell.value = val;
            if (c.v?.bl) cell.font = { ...cell.font, bold: true };
            if (c.v?.it) cell.font = { ...cell.font, italic: true };
          }
        }
      }
      const buf = await wb.xlsx.writeBuffer();
      await doSave(userId, filePath, fileName, new Blob([buf], {
        type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
      }));
    } catch (e) {
      Toast.error('保存失败: ' + e.message);
    } finally {
      setSaving(false);
    }
  }, [userId, filePath, fileName]);

  if (err) return <Text type="danger">Excel 渲染失败: {err}</Text>;
  if (!content) return <div style={{ display: 'flex', justifyContent: 'center', paddingTop: 40 }}><Spin /></div>;

  const { Workbook, sheets } = content;
  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      {userId && <Toolbar onSave={handleSave} saving={saving} />}
      <div style={{ flex: 1, minHeight: 0 }}>
        <Workbook
          data={sheets}
          allowEdit={true}
          onChange={(d) => { sheetsRef.current = d; }}
        />
      </div>
    </div>
  );
}

// ── PPTX 工具常量 ──
const EMU_PER_PX = 914400 / 96; // 9525
const NS = { a: 'http://schemas.openxmlformats.org/drawingml/2006/main', p: 'http://schemas.openxmlformats.org/presentationml/2006/main', r: 'http://schemas.openxmlformats.org/officeDocument/2006/relationships' };
const emu2px = (emu) => Math.round(Number(emu) / EMU_PER_PX);
const szToPt = (sz) => Number(sz) / 100;

// 解析幻灯片尺寸
async function parsePresentationDimensions(zip) {
  const presFile = zip.files['ppt/presentation.xml'];
  if (!presFile) return { w: 960, h: 720 };
  const xml = await presFile.async('text');
  const doc = new DOMParser().parseFromString(xml, 'application/xml');
  const sldSz = doc.getElementsByTagName('p:sldSz')[0];
  if (!sldSz) return { w: 960, h: 720 };
  return { w: emu2px(sldSz.getAttribute('cx') || 9144000), h: emu2px(sldSz.getAttribute('cy') || 6858000) };
}

// 辅助：从 XML 元素中递归查找指定 tag（忽略命名空间前缀）
function findTag(el, localName) {
  if (!el) return null;
  // 先尝试 NS 方式
  for (const ns of Object.values(NS)) {
    const found = el.getElementsByTagNameNS(ns, localName);
    if (found.length) return found[0];
  }
  // 回退：带前缀搜索
  for (const prefix of ['a:', 'p:', 'r:', '']) {
    const found = el.getElementsByTagName(prefix + localName);
    if (found.length) return found[0];
  }
  return null;
}

// 解析颜色值（<a:solidFill> 下的 <a:srgbClr> 或 <a:schemeClr>）
function parseSolidFill(el) {
  if (!el) return null;
  const sf = findTag(el, 'solidFill');
  if (!sf) return null;
  const srgb = findTag(sf, 'srgbClr');
  if (srgb) return '#' + srgb.getAttribute('val');
  return null;
}

// 解析单个幻灯片
async function parseSlide(zip, xmlPath, slideIdx) {
  const xml = await zip.files[xmlPath].async('text');
  const doc = new DOMParser().parseFromString(xml, 'application/xml');

  // 解析 rels 文件获取图片映射
  const relPath = xmlPath.replace('ppt/slides/', 'ppt/slides/_rels/') + '.rels';
  const imgMap = {};
  if (zip.files[relPath]) {
    const relXml = await zip.files[relPath].async('text');
    const relDoc = new DOMParser().parseFromString(relXml, 'application/xml');
    const rels = relDoc.getElementsByTagName('Relationship');
    for (let i = 0; i < rels.length; i++) {
      const rel = rels[i];
      const target = rel.getAttribute('Target') || '';
      if (/\.(png|jpg|jpeg|gif|bmp|svg|webp|tiff?)$/i.test(target)) {
        imgMap[rel.getAttribute('Id')] = 'ppt/' + target.replace('../', '');
      }
    }
  }

  // 背景色
  const bgEl = doc.getElementsByTagName('p:bg')[0];
  const bgColor = parseSolidFill(bgEl);

  const shapes = [];
  const images = [];
  let runIdx = 0;

  // 遍历幻灯片树中的所有 shape
  const spTree = doc.getElementsByTagName('p:spTree')[0];
  if (!spTree) return { shapes, images, bgColor, xmlPath };

  // 处理图片 <p:pic>
  const pics = spTree.getElementsByTagName('p:pic');
  for (let i = 0; i < pics.length; i++) {
    const pic = pics[i];
    const xfrm = findTag(pic, 'xfrm');
    if (!xfrm) continue;
    const off = findTag(xfrm, 'off');
    const ext = findTag(xfrm, 'ext');
    if (!off || !ext) continue;
    // 找 blipFill → blip 的 r:embed
    const blip = findTag(pic, 'blip');
    if (!blip) continue;
    const rId = blip.getAttribute('r:embed') || blip.getAttributeNS(NS.r, 'embed');
    const imgZipPath = imgMap[rId];
    if (!imgZipPath || !zip.files[imgZipPath]) continue;
    const imgBlob = await zip.files[imgZipPath].async('blob');
    images.push({
      x: emu2px(off.getAttribute('x') || 0),
      y: emu2px(off.getAttribute('y') || 0),
      w: emu2px(ext.getAttribute('cx') || 0),
      h: emu2px(ext.getAttribute('cy') || 0),
      url: URL.createObjectURL(imgBlob),
    });
  }

  // 处理文字形状 <p:sp>
  const sps = spTree.getElementsByTagName('p:sp');
  for (let i = 0; i < sps.length; i++) {
    const sp = sps[i];
    const xfrm = findTag(sp, 'xfrm');
    if (!xfrm) continue; // 跳过无位置信息的占位符
    const off = findTag(xfrm, 'off');
    const ext = findTag(xfrm, 'ext');
    if (!off || !ext) continue;

    const shapeX = emu2px(off.getAttribute('x') || 0);
    const shapeY = emu2px(off.getAttribute('y') || 0);
    const shapeW = emu2px(ext.getAttribute('cx') || 0);
    const shapeH = emu2px(ext.getAttribute('cy') || 0);

    // 解析段落
    const txBody = findTag(sp, 'txBody');
    if (!txBody) continue;
    const paragraphs = [];
    const pEls = txBody.getElementsByTagName('a:p');
    if (pEls.length === 0) continue;

    let hasText = false;
    for (let pi = 0; pi < pEls.length; pi++) {
      const pEl = pEls[pi];
      // 段落对齐
      const pPr = pEl.getElementsByTagName('a:pPr')[0];
      const algn = pPr?.getAttribute('algn');
      const textAlign = algn === 'ctr' ? 'center' : algn === 'r' ? 'right' : algn === 'just' ? 'justify' : 'left';

      const runs = [];
      const rEls = pEl.getElementsByTagName('a:r');
      for (let ri = 0; ri < rEls.length; ri++) {
        const rEl = rEls[ri];
        const tEl = rEl.getElementsByTagName('a:t')[0];
        if (!tEl) continue;
        const text = tEl.textContent || '';
        if (text) hasText = true;

        // 格式属性
        const rPr = rEl.getElementsByTagName('a:rPr')[0];
        const bold = rPr?.getAttribute('b') === '1';
        const italic = rPr?.getAttribute('i') === '1';
        const fontSize = rPr?.getAttribute('sz') ? szToPt(rPr.getAttribute('sz')) : null;
        const color = parseSolidFill(rPr);

        runs.push({ text, bold, italic, fontSize, color, slideIdx, runIdx: runIdx++, originalText: text });
      }
      // 也处理 <a:fld> 中的文本（日期/页码等占位符）
      const fldEls = pEl.getElementsByTagName('a:fld');
      for (let fi = 0; fi < fldEls.length; fi++) {
        const fEl = fldEls[fi];
        const tEl = fEl.getElementsByTagName('a:t')[0];
        if (!tEl) continue;
        const text = tEl.textContent || '';
        if (text) hasText = true;
        runs.push({ text, bold: false, italic: false, fontSize: null, color: null, slideIdx, runIdx: runIdx++, originalText: text, isField: true });
      }
      paragraphs.push({ textAlign, runs });
    }
    if (!hasText) continue;
    shapes.push({ x: shapeX, y: shapeY, w: shapeW, h: shapeH, paragraphs });
  }

  return { shapes, images, bgColor, xmlPath };
}

// 解析所有幻灯片
async function parseAllSlides(zip) {
  const slideFiles = Object.keys(zip.files)
    .filter(f => /^ppt\/slides\/slide\d+\.xml$/.test(f))
    .sort((a, b) => {
      const na = parseInt(a.match(/slide(\d+)/)[1]);
      const nb = parseInt(b.match(/slide(\d+)/)[1]);
      return na - nb;
    });
  const dims = await parsePresentationDimensions(zip);
  const slides = [];
  for (let i = 0; i < slideFiles.length; i++) {
    slides.push(await parseSlide(zip, slideFiles[i], i));
  }
  return { dims, slides };
}

// 保存时修改 XML 中的 <a:t> 文本
function updateSlideXml(xmlString, runMods) {
  const doc = new DOMParser().parseFromString(xmlString, 'application/xml');
  // 收集所有 <a:t> 元素（按文档顺序）
  const allT = [];
  const tFromR = doc.getElementsByTagName('a:r');
  for (let i = 0; i < tFromR.length; i++) {
    const t = tFromR[i].getElementsByTagName('a:t')[0];
    if (t) allT.push(t);
  }
  const tFromFld = doc.getElementsByTagName('a:fld');
  for (let i = 0; i < tFromFld.length; i++) {
    const t = tFromFld[i].getElementsByTagName('a:t')[0];
    if (t) allT.push(t);
  }
  // 按 runIdx 排序不可行（无属性），所以用全局计数器重建与 parseSlide 一致的顺序
  // 更精确的方式：遍历 <p:sp> → <a:p> → <a:r>|<a:fld> → <a:t> 顺序
  const orderedT = [];
  const spTree = doc.getElementsByTagName('p:spTree')[0];
  if (spTree) {
    const sps = spTree.getElementsByTagName('p:sp');
    for (let si = 0; si < sps.length; si++) {
      const txBody = sps[si].getElementsByTagName('a:txBody')[0];
      if (!txBody) continue;
      const ps = txBody.getElementsByTagName('a:p');
      for (let pi = 0; pi < ps.length; pi++) {
        const rs = ps[pi].getElementsByTagName('a:r');
        for (let ri = 0; ri < rs.length; ri++) {
          const t = rs[ri].getElementsByTagName('a:t')[0];
          if (t) orderedT.push(t);
        }
        const flds = ps[pi].getElementsByTagName('a:fld');
        for (let fi = 0; fi < flds.length; fi++) {
          const t = flds[fi].getElementsByTagName('a:t')[0];
          if (t) orderedT.push(t);
        }
      }
    }
  }
  for (const [localIdx, newText] of runMods) {
    if (localIdx >= 0 && localIdx < orderedT.length) {
      orderedT[localIdx].textContent = newText;
    }
  }
  return new XMLSerializer().serializeToString(doc);
}

// PPTX 编辑器（替代 PptxRenderer）
function PptxEditor({ blob, userId, filePath, fileName }) {
  const [slides, setSlides] = useState([]);
  const [dims, setDims] = useState({ w: 960, h: 720 });
  const [current, setCurrent] = useState(0);
  const [err, setErr] = useState(null);
  const [saving, setSaving] = useState(false);
  const zipRef = useRef(null);
  const modifiedRunsRef = useRef(new Map()); // key: "slideIdx:runIdx", value: newText
  const [hasModifications, setHasModifications] = useState(false);
  const imgUrlsRef = useRef([]); // 跟踪所有 object URL 以便清理

  useEffect(() => {
    if (!blob) return;
    let cancelled = false;
    (async () => {
      try {
        const buf = await blob.arrayBuffer();
        if (buf.byteLength < 100) throw new Error(`文件太小 (${buf.byteLength} bytes)，不是有效的 PPTX`);
        const header = new Uint8Array(buf.slice(0, 16));
        const headStr = new TextDecoder().decode(header);
        if (headStr.startsWith('<!') || headStr.startsWith('<html') || headStr.startsWith('{"')) {
          throw new Error('服务器返回了错误页面而非文件内容，请确认后端已部署 preview 接口');
        }
        if (header[0] !== 0x50 || header[1] !== 0x4B) {
          const hex = Array.from(header.slice(0, 8)).map(b => b.toString(16).padStart(2, '0')).join(' ');
          throw new Error(`非 ZIP 格式 (头部: ${hex})，可能是旧版 .ppt 文件`);
        }
        const JSZip = (await import('jszip')).default;
        let zip;
        try {
          zip = await JSZip.loadAsync(buf);
        } catch (zipErr) {
          throw new Error(`ZIP 解析失败 (${(buf.byteLength / 1024).toFixed(1)}KB): ${zipErr.message}`);
        }
        zipRef.current = zip;
        const result = await parseAllSlides(zip);
        if (!cancelled) {
          // 收集所有图片 URL 以便 cleanup
          const urls = [];
          for (const s of result.slides) {
            for (const img of s.images) urls.push(img.url);
          }
          imgUrlsRef.current = urls;
          setDims(result.dims);
          setSlides(result.slides);
        }
      } catch (e) {
        if (!cancelled) setErr(e.message);
      }
    })();
    return () => {
      cancelled = true;
      // 清理 object URLs
      for (const url of imgUrlsRef.current) URL.revokeObjectURL(url);
      imgUrlsRef.current = [];
    };
  }, [blob]);

  // 文字编辑 blur 回调
  const handleRunBlur = useCallback((slideIdx, runIdx, originalText, e) => {
    const newText = e.target.textContent || '';
    const key = `${slideIdx}:${runIdx}`;
    if (newText !== originalText) {
      modifiedRunsRef.current.set(key, newText);
    } else {
      modifiedRunsRef.current.delete(key);
    }
    setHasModifications(modifiedRunsRef.current.size > 0);
  }, []);

  // 粘贴时强制纯文本
  const handlePaste = useCallback((e) => {
    e.preventDefault();
    const text = e.clipboardData.getData('text/plain');
    document.execCommand('insertText', false, text);
  }, []);

  // 保存
  const handleSave = useCallback(async () => {
    if (!zipRef.current || !userId || modifiedRunsRef.current.size === 0) return;
    setSaving(true);
    try {
      const zip = zipRef.current;
      // 按 slideIdx 分组
      const bySlide = new Map();
      for (const [key, newText] of modifiedRunsRef.current) {
        const [si, ri] = key.split(':').map(Number);
        if (!bySlide.has(si)) bySlide.set(si, []);
        bySlide.get(si).push([ri, newText]);
      }
      // 计算每个 slide 内的局部 runIdx 偏移
      // 每个 slide 解析时 runIdx 从该 slide 起始位置开始计数
      // 需要知道每个 slide 的起始 runIdx
      let slideRunStart = 0;
      for (let si = 0; si < slides.length; si++) {
        const slide = slides[si];
        let slideRunCount = 0;
        for (const shape of slide.shapes) {
          for (const para of shape.paragraphs) {
            slideRunCount += para.runs.length;
          }
        }
        if (bySlide.has(si)) {
          const mods = bySlide.get(si);
          // 转换为 slide 内部的局部索引
          const localMods = mods.map(([globalRunIdx, text]) => [globalRunIdx - slideRunStart, text]);
          const xmlPath = slide.xmlPath;
          const origXml = await zip.files[xmlPath].async('text');
          const updatedXml = updateSlideXml(origXml, localMods);
          zip.file(xmlPath, updatedXml);
        }
        slideRunStart += slideRunCount;
      }
      const newBlob = await zip.generateAsync({
        type: 'blob',
        mimeType: 'application/vnd.openxmlformats-officedocument.presentationml.presentation',
      });
      await doSave(userId, filePath, fileName, newBlob);
      // 保存成功后，更新 originalText 并清空 modifiedRuns
      modifiedRunsRef.current.clear();
      setHasModifications(false);
      // 更新 slides 中的 originalText
      setSlides(prev => prev.map(slide => ({
        ...slide,
        shapes: slide.shapes.map(shape => ({
          ...shape,
          paragraphs: shape.paragraphs.map(para => ({
            ...para,
            runs: para.runs.map(run => ({ ...run, originalText: run.text })),
          })),
        })),
      })));
    } catch (e) {
      Toast.error('保存失败: ' + e.message);
    } finally {
      setSaving(false);
    }
  }, [userId, filePath, fileName, slides]);

  if (err) return <Text type="danger">PPTX 渲染失败: {err}</Text>;
  if (slides.length === 0) return <div style={{ display: 'flex', justifyContent: 'center', paddingTop: 40 }}><Spin /></div>;

  const slide = slides[current];
  const VIEWPORT_W = 528;
  const scale = VIEWPORT_W / dims.w;
  const viewportH = Math.round(dims.h * scale);

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      {userId && (
        <Toolbar onSave={handleSave} saving={saving} extra={
          hasModifications ? <Text type="warning" size="small" style={{ fontSize: 12 }}>有未保存的修改</Text> : null
        } />
      )}
      <div style={{ flex: 1, overflow: 'auto', padding: 16, background: '#f5f5f5', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
        {/* 缩放视口 */}
        <div style={{
          width: VIEWPORT_W, height: viewportH, overflow: 'hidden', position: 'relative',
          borderRadius: 4, boxShadow: '0 2px 8px rgba(0,0,0,0.12)',
        }}>
          {/* 内层：原始尺寸，CSS scale 缩放 */}
          <div style={{
            width: dims.w, height: dims.h, position: 'relative',
            transform: `scale(${scale})`, transformOrigin: 'top left',
            background: slide.bgColor || '#fff',
          }}>
            {/* 图片 */}
            {slide.images.map((img, i) => (
              <img key={`img-${i}`} src={img.url} alt=""
                style={{ position: 'absolute', left: img.x, top: img.y, width: img.w, height: img.h, objectFit: 'contain', pointerEvents: 'none' }}
              />
            ))}
            {/* 文字形状 */}
            {slide.shapes.map((shape, si) => (
              <div key={`shape-${si}`} style={{
                position: 'absolute', left: shape.x, top: shape.y, width: shape.w, height: shape.h,
                overflow: 'hidden', boxSizing: 'border-box', padding: '4px 8px',
              }}>
                {shape.paragraphs.map((para, pi) => (
                  <div key={`p-${pi}`} style={{ textAlign: para.textAlign, margin: 0, lineHeight: 1.3 }}>
                    {para.runs.map((run, ri) => (
                      <span
                        key={`r-${ri}`}
                        contentEditable={!run.isField && !!userId}
                        suppressContentEditableWarning
                        onBlur={(e) => handleRunBlur(run.slideIdx, run.runIdx, run.originalText, e)}
                        onPaste={handlePaste}
                        style={{
                          fontWeight: run.bold ? 'bold' : 'normal',
                          fontStyle: run.italic ? 'italic' : 'normal',
                          fontSize: run.fontSize ? `${run.fontSize}pt` : '12pt',
                          color: run.color || '#000',
                          outline: 'none',
                          borderBottom: '1px solid transparent',
                          transition: 'border-color 0.2s, background 0.2s',
                          cursor: !run.isField && userId ? 'text' : 'default',
                        }}
                        onFocus={(e) => {
                          if (!run.isField && userId) {
                            e.target.style.borderBottom = '1px dashed #1890ff';
                            e.target.style.background = 'rgba(24,144,255,0.06)';
                          }
                        }}
                        onBlurCapture={(e) => {
                          e.target.style.borderBottom = '1px solid transparent';
                          e.target.style.background = 'transparent';
                        }}
                      >
                        {run.text}
                      </span>
                    ))}
                  </div>
                ))}
              </div>
            ))}
            {slide.shapes.length === 0 && slide.images.length === 0 && (
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', width: '100%', height: '100%' }}>
                <Text type="tertiary">（空白幻灯片）</Text>
              </div>
            )}
          </div>
        </div>
      </div>
      {slides.length > 1 && (
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 12, padding: 8, borderTop: '1px solid var(--semi-color-border)', flexShrink: 0 }}>
          <button onClick={() => setCurrent(Math.max(0, current - 1))} disabled={current === 0}
            style={{ padding: '4px 12px', cursor: 'pointer', border: '1px solid var(--semi-color-border)', borderRadius: 4, background: 'transparent' }}>上一页</button>
          <span style={{ fontSize: 13 }}>{current + 1} / {slides.length}</span>
          <button onClick={() => setCurrent(Math.min(slides.length - 1, current + 1))} disabled={current === slides.length - 1}
            style={{ padding: '4px 12px', cursor: 'pointer', border: '1px solid var(--semi-color-border)', borderRadius: 4, background: 'transparent' }}>下一页</button>
        </div>
      )}
    </div>
  );
}

// 代码编辑器（懒加载）
const CodeMirrorEditor = React.lazy(() => import('@uiw/react-codemirror'));

async function getLanguage(ext) {
  switch (ext) {
    case 'js': case 'jsx': case 'ts': case 'tsx': case 'vue': case 'svelte': {
      const { javascript } = await import('@codemirror/lang-javascript');
      return javascript({ jsx: ['jsx','tsx','vue','svelte'].includes(ext), typescript: ['ts','tsx'].includes(ext) });
    }
    case 'py': { const { python } = await import('@codemirror/lang-python'); return python(); }
    case 'java': case 'kt': { const { java } = await import('@codemirror/lang-java'); return java(); }
    case 'c': case 'cpp': case 'h': { const { cpp } = await import('@codemirror/lang-cpp'); return cpp(); }
    case 'rs': { const { rust } = await import('@codemirror/lang-rust'); return rust(); }
    case 'css': case 'scss': case 'sass': case 'less': { const { css } = await import('@codemirror/lang-css'); return css(); }
    case 'html': case 'htm': { const { html } = await import('@codemirror/lang-html'); return html(); }
    case 'json': { const { json } = await import('@codemirror/lang-json'); return json(); }
    case 'md': { const { markdown } = await import('@codemirror/lang-markdown'); return markdown(); }
    case 'sql': { const { sql } = await import('@codemirror/lang-sql'); return sql(); }
    case 'xml': case 'yaml': case 'yml': case 'toml': { const { xml } = await import('@codemirror/lang-xml'); return xml(); }
    default: return null;
  }
}

const RENDERABLE_EXTS = new Set(['html', 'htm', 'md']);

function CodeEditor({ data, userId, filePath, fileName, ext }) {
  const [code, setCode] = useState(data);
  const [lang, setLang] = useState([]);
  const [saving, setSaving] = useState(false);
  const [renderMode, setRenderMode] = useState(false);
  const isRenderable = RENDERABLE_EXTS.has(ext);

  useEffect(() => { getLanguage(ext).then(l => l && setLang([l])); }, [ext]);

  const handleSave = useCallback(async () => {
    if (!userId) return;
    setSaving(true);
    try {
      await doSave(userId, filePath, fileName, new Blob([code], { type: 'text/plain' }));
    } catch (e) { Toast.error('保存失败: ' + e.message); }
    finally { setSaving(false); }
  }, [code, userId, filePath, fileName]);

  const renderToggle = isRenderable ? (
    <Button
      size="small"
      theme={renderMode ? 'solid' : 'light'}
      onClick={() => setRenderMode(!renderMode)}
      style={{ marginLeft: 4 }}
    >
      {renderMode ? '源码' : '渲染'}
    </Button>
  ) : null;

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      {userId && <Toolbar onSave={handleSave} saving={saving} extra={renderToggle} />}
      {!userId && isRenderable && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '6px 8px', borderBottom: '1px solid var(--semi-color-border)', flexShrink: 0 }}>
          {renderToggle}
        </div>
      )}
      {renderMode ? (
        <RenderedView code={code} ext={ext} />
      ) : (
        <Suspense fallback={<div style={{ padding: 16 }}>加载编辑器...</div>}>
          <CodeMirrorEditor
            value={code}
            extensions={lang}
            onChange={setCode}
            height="100%"
            style={{ flex: 1, overflow: 'auto', fontSize: 13 }}
            basicSetup={{ lineNumbers: true, foldGutter: true, highlightActiveLine: true }}
          />
        </Suspense>
      )}
    </div>
  );
}

// 渲染视图组件
function RenderedView({ code, ext }) {
  if (ext === 'md') {
    return (
      <div
        className="message-text"
        style={{
          flex: 1, overflow: 'auto', padding: '20px 24px',
          fontFamily: '"Segoe UI", "Microsoft YaHei", sans-serif',
          fontSize: 14, lineHeight: 1.8,
          background: 'var(--semi-color-bg-0)', color: 'var(--semi-color-text-0)',
          minHeight: 0,
        }}
      >
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{code}</ReactMarkdown>
      </div>
    );
  }

  // HTML 渲染使用 iframe srcdoc 进行沙箱隔离
  return (
    <iframe
      srcDoc={code}
      title="HTML Preview"
      sandbox="allow-same-origin allow-scripts"
      style={{
        flex: 1, width: '100%', border: 'none',
        background: '#fff', minHeight: 0,
      }}
    />
  );
}

// 文本编辑器
function TextEditor({ data, userId, filePath, fileName }) {
  const [text, setText] = useState(data);
  const [saving, setSaving] = useState(false);

  const handleSave = useCallback(async () => {
    if (!userId) return;
    setSaving(true);
    try {
      await doSave(userId, filePath, fileName, new Blob([text], { type: 'text/plain' }));
    } catch (e) {
      Toast.error('保存失败: ' + e.message);
    } finally {
      setSaving(false);
    }
  }, [text, userId, filePath, fileName]);

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      {userId && <Toolbar onSave={handleSave} saving={saving} />}
      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        style={{
          flex: 1, margin: 0, padding: 8, fontSize: 12, border: 'none', resize: 'none',
          fontFamily: 'Consolas, "Courier New", monospace', outline: 'none',
          whiteSpace: 'pre', overflow: 'auto', background: 'var(--semi-color-bg-1)',
          color: 'var(--semi-color-text-0)', minHeight: 0,
        }}
        spellCheck={false}
      />
    </div>
  );
}

// 主组件
const FilePreview = ({ previewUrl, fileName, userId, filePath }) => {
  const [content, setContent] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const blobUrlRef = useRef(null);
  const ext = getExt(fileName);

  useEffect(() => {
    if (!previewUrl) return;
    setLoading(true);
    setError(null);
    setContent(null);
    if (blobUrlRef.current) { URL.revokeObjectURL(blobUrlRef.current); blobUrlRef.current = null; }

    const doFetch = async () => {
      try {
        const res = await fetch(previewUrl, { credentials: 'same-origin' });
        if (!res.ok) throw new Error(`HTTP ${res.status}: ${res.statusText}`);

        // 文本类
        if (TEXT_EXTS.has(ext) || CODE_EXTS.has(ext)) {
          setContent({ type: 'text', data: await res.text() });
          setLoading(false);
          return;
        }

        const blob = await res.blob();
        console.log('[FilePreview] blob size:', blob.size, 'type:', blob.type, 'file:', fileName);
        if (blob.size === 0) throw new Error('文件内容为空 (0 bytes)');
        // 检测后端是否返回了 HTML 错误页面（如 SPA fallback）
        if (blob.type && blob.type.includes('text/html') && !['html','htm'].includes(ext)) {
          throw new Error('服务器返回了 HTML 页面而非文件内容，请确认后端 preview 接口已部署并重启');
        }

        // 需要 blob URL 的类型
        if (IMG_EXTS.has(ext) || ext === 'pdf' || VIDEO_EXTS.has(ext) || AUDIO_EXTS.has(ext)) {
          const url = URL.createObjectURL(blob);
          blobUrlRef.current = url;
          const type = ext === 'pdf' ? 'pdf' : IMG_EXTS.has(ext) ? 'image' : VIDEO_EXTS.has(ext) ? 'video' : 'audio';
          setContent({ type, url });
          setLoading(false);
          return;
        }

        // 旧版 Office 格式（OLE2）不支持前端渲染
        if (ext === 'doc' || ext === 'xls' || ext === 'ppt') {
          setLoading(false);
          setError(`unsupported_legacy`);
          return;
        }

        // 需要 blob 对象的类型（docx/xlsx/pptx）
        if (ext === 'docx') { setContent({ type: 'docx', blob }); setLoading(false); return; }
        if (ext === 'xlsx') { setContent({ type: 'xlsx', blob }); setLoading(false); return; }
        if (ext === 'pptx') { setContent({ type: 'pptx', blob }); setLoading(false); return; }

        setLoading(false);
        setError('unsupported');
      } catch (e) {
        console.error('[FilePreview]', e);
        setError(e.message);
        setLoading(false);
      }
    };
    doFetch();
    return () => { if (blobUrlRef.current) { URL.revokeObjectURL(blobUrlRef.current); blobUrlRef.current = null; } };
  }, [previewUrl, ext]);

  if (loading) return <div style={{ display: 'flex', justifyContent: 'center', paddingTop: 40 }}><Spin /></div>;
  if (error === 'unsupported_legacy') return <Text type="tertiary">旧版 Office 格式 (.{ext}) 不支持在线预览，请转换为 .docx/.xlsx/.pptx 格式</Text>;
  if (error && error !== 'unsupported') return <Text type="danger">加载失败: {error}</Text>;

  if (content?.type === 'image') return <img src={content.url} alt={fileName} style={{ maxWidth: '100%', maxHeight: '100%', objectFit: 'contain' }} />;
  if (content?.type === 'pdf') return <iframe src={content.url} title={fileName} style={{ width: '100%', height: '100%', border: 'none' }} />;
  if (content?.type === 'video') return <video src={content.url} controls style={{ maxWidth: '100%', maxHeight: '100%' }} />;
  if (content?.type === 'audio') return <audio src={content.url} controls style={{ width: '100%', marginTop: 20 }} />;
  if (content?.type === 'text') return <CodeEditor data={content.data} userId={userId} filePath={filePath} fileName={fileName} ext={ext} />;
  if (content?.type === 'docx') return <DocxEditor blob={content.blob} userId={userId} filePath={filePath} fileName={fileName} />;
  if (content?.type === 'xlsx') return <XlsxEditor blob={content.blob} userId={userId} filePath={filePath} fileName={fileName} />;
  if (content?.type === 'pptx') return <PptxEditor blob={content.blob} userId={userId} filePath={filePath} fileName={fileName} />;

  return <Text type="tertiary">不支持预览此文件类型 (.{ext})</Text>;
};

export default FilePreview;
