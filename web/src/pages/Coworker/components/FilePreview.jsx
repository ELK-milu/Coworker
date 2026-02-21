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

// PPTX 渲染器
function PptxRenderer({ blob }) {
  const [slides, setSlides] = useState([]);
  const [current, setCurrent] = useState(0);
  const [err, setErr] = useState(null);

  useEffect(() => {
    if (!blob) return;
    let cancelled = false;
    (async () => {
      try {
        const buf = await blob.arrayBuffer();
        console.log('[PptxRenderer] arrayBuffer size:', buf.byteLength, 'blob type:', blob.type);
        if (buf.byteLength < 100) throw new Error(`文件太小 (${buf.byteLength} bytes)，不是有效的 PPTX`);
        const header = new Uint8Array(buf.slice(0, 16));
        // 检测是否为 HTML 错误页面
        const headStr = new TextDecoder().decode(header);
        if (headStr.startsWith('<!') || headStr.startsWith('<html') || headStr.startsWith('{"')) {
          throw new Error('服务器返回了错误页面而非文件内容，请确认后端已部署 preview 接口');
        }
        // 检查 ZIP 魔数 (PK\x03\x04)
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
        const slideFiles = Object.keys(zip.files)
          .filter(f => /^ppt\/slides\/slide\d+\.xml$/.test(f))
          .sort((a, b) => {
            const na = parseInt(a.match(/slide(\d+)/)[1]);
            const nb = parseInt(b.match(/slide(\d+)/)[1]);
            return na - nb;
          });

        const parsed = [];
        for (const sf of slideFiles) {
          const xml = await zip.files[sf].async('text');
          // 提取文本内容
          const texts = [];
          const textMatches = xml.matchAll(/<a:t>([\s\S]*?)<\/a:t>/g);
          for (const m of textMatches) texts.push(m[1]);

          // 提取图片引用
          const images = [];
          const relFile = sf.replace('ppt/slides/', 'ppt/slides/_rels/') + '.rels';
          if (zip.files[relFile]) {
            const relXml = await zip.files[relFile].async('text');
            const imgRels = relXml.matchAll(/Target="([^"]*\.(png|jpg|jpeg|gif|bmp|svg|webp))"/gi);
            for (const m of imgRels) {
              const imgPath = 'ppt/' + m[1].replace('../', '');
              if (zip.files[imgPath]) {
                const imgBlob = await zip.files[imgPath].async('blob');
                images.push(URL.createObjectURL(imgBlob));
              }
            }
          }
          parsed.push({ texts, images });
        }
        if (!cancelled) setSlides(parsed);
      } catch (e) {
        if (!cancelled) setErr(e.message);
      }
    })();
    return () => { cancelled = true; };
  }, [blob]);

  if (err) return <Text type="danger">PPTX 渲染失败: {err}</Text>;
  if (slides.length === 0) return <div style={{ display: 'flex', justifyContent: 'center', paddingTop: 40 }}><Spin /></div>;

  const slide = slides[current];
  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      <div style={{ flex: 1, overflow: 'auto', padding: 16, background: '#f5f5f5', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
        <div style={{ background: '#fff', padding: 24, borderRadius: 4, boxShadow: '0 1px 4px rgba(0,0,0,0.1)', width: '100%', maxWidth: 600, minHeight: 300 }}>
          {slide.images.map((src, i) => (
            <img key={i} src={src} alt="" style={{ maxWidth: '100%', marginBottom: 8 }} />
          ))}
          {slide.texts.map((t, i) => (
            <p key={i} style={{ margin: '4px 0', fontSize: 14 }}>{t}</p>
          ))}
          {slide.texts.length === 0 && slide.images.length === 0 && (
            <Text type="tertiary">（空白幻灯片）</Text>
          )}
        </div>
      </div>
      {slides.length > 1 && (
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 12, padding: 8, borderTop: '1px solid var(--semi-color-border)', flexShrink: 0 }}>
          <button onClick={() => setCurrent(Math.max(0, current - 1))} disabled={current === 0}
            style={{ padding: '4px 12px', cursor: 'pointer', border: '1px solid var(--semi-color-border)', borderRadius: 4 }}>上一页</button>
          <span style={{ fontSize: 13 }}>{current + 1} / {slides.length}</span>
          <button onClick={() => setCurrent(Math.min(slides.length - 1, current + 1))} disabled={current === slides.length - 1}
            style={{ padding: '4px 12px', cursor: 'pointer', border: '1px solid var(--semi-color-border)', borderRadius: 4 }}>下一页</button>
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
  if (content?.type === 'pptx') return <PptxRenderer blob={content.blob} />;

  return <Text type="tertiary">不支持预览此文件类型 (.{ext})</Text>;
};

export default FilePreview;
