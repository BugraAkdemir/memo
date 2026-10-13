// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Injected into the chat page by driver.go as (function(cfg, op, arg){...})(...).
// Everything that touches the page's DOM lives here, in one function, for two
// reasons: one round trip per poll, and exactly one place to fix when a site
// moves things around. It always returns a JSON string and never throws — a
// failure inside the page must come back as data the Go side can classify,
// not as a protocol error.
(function (cfg, op, arg) {
  'use strict';

  function visible(el) {
    if (!el || !el.getBoundingClientRect) return false;
    var r = el.getBoundingClientRect();
    if (r.width <= 0 || r.height <= 0) return false;
    var st = window.getComputedStyle(el);
    return st.visibility !== 'hidden' && st.display !== 'none';
  }

  // first element matching the first selector (in list order) that has any
  // visible match; the matching selector is returned so a diagnosis can say
  // which candidate is doing the work.
  function pickVisible(sels) {
    for (var i = 0; i < (sels || []).length; i++) {
      var list;
      try { list = document.querySelectorAll(sels[i]); } catch (e) { continue; }
      for (var j = 0; j < list.length; j++) {
        if (visible(list[j])) return { el: list[j], sel: sels[i] };
      }
    }
    return null;
  }

  function anyVisible(sels) { return pickVisible(sels) !== null; }

  function allMatches(sels) {
    for (var i = 0; i < (sels || []).length; i++) {
      var list;
      try { list = document.querySelectorAll(sels[i]); } catch (e) { continue; }
      if (list.length) return { list: Array.prototype.slice.call(list), sel: sels[i] };
    }
    return null;
  }

  function re(src) {
    if (!src) return null;
    try { return new RegExp(src, 'i'); } catch (e) { return null; }
  }

  // ---- rendered DOM -> markdown ------------------------------------------
  // innerText flattens code fences, lists and tables into prose, which makes
  // the reply unreadable once Memo renders it as markdown again.
  var SKIP = { SCRIPT: 1, STYLE: 1, BUTTON: 1, SVG: 1, NOSCRIPT: 1, TEMPLATE: 1 };

  function inline(node) {
    var out = '';
    for (var c = node.firstChild; c; c = c.nextSibling) out += md(c, 0);
    return out;
  }

  function tableToMd(t) {
    var rows = Array.prototype.slice.call(t.querySelectorAll('tr')).map(function (tr) {
      return Array.prototype.slice.call(tr.children).map(function (td) {
        return inline(td).replace(/\s*\n\s*/g, ' ').replace(/\|/g, '\\|').trim();
      });
    });
    if (!rows.length) return '';
    var width = Math.max.apply(null, rows.map(function (r) { return r.length; }));
    function pad(r) { while (r.length < width) r.push(''); return '| ' + r.join(' | ') + ' |'; }
    var lines = [pad(rows[0]), '|' + new Array(width + 1).join(' --- |')];
    for (var i = 1; i < rows.length; i++) lines.push(pad(rows[i]));
    return '\n\n' + lines.join('\n') + '\n\n';
  }

  function md(node, depth) {
    if (node.nodeType === 3) return node.nodeValue.replace(/\s+/g, ' ');
    if (node.nodeType !== 1) return '';
    var tag = node.tagName.toUpperCase();
    if (SKIP[tag]) return '';
    if (node.classList && node.classList.contains('katex-html')) return '';
    if (node.classList && node.classList.contains('katex')) {
      var ann = node.querySelector('annotation[encoding="application/x-tex"]');
      if (ann) {
        var disp = node.closest('.katex-display');
        return disp ? '\n\n$$' + ann.textContent + '$$\n\n' : '$' + ann.textContent + '$';
      }
    }
    switch (tag) {
      case 'PRE': {
        var code = node.querySelector('code');
        var text = (code || node).textContent.replace(/\n$/, '');
        var lang = '';
        if (code) {
          var m = /(?:^|\s)language-([\w+#.-]+)/.exec(code.className || '');
          if (m) lang = m[1];
        }
        return '\n\n```' + lang + '\n' + text + '\n```\n\n';
      }
      case 'CODE': return '`' + node.textContent + '`';
      case 'H1': case 'H2': case 'H3': case 'H4': case 'H5': case 'H6':
        return '\n\n' + new Array(parseInt(tag[1], 10) + 1).join('#') + ' ' + inline(node).trim() + '\n\n';
      case 'P': return '\n\n' + inline(node).trim() + '\n\n';
      case 'BR': return '\n';
      case 'HR': return '\n\n---\n\n';
      case 'STRONG': case 'B': { var s = inline(node); return s.trim() ? '**' + s.trim() + '**' : s; }
      case 'EM': case 'I': { var e = inline(node); return e.trim() ? '*' + e.trim() + '*' : e; }
      case 'DEL': case 'S': return '~~' + inline(node).trim() + '~~';
      case 'A': {
        var href = node.getAttribute('href') || '';
        var label = inline(node).trim();
        return href && label ? '[' + label + '](' + href + ')' : label;
      }
      case 'IMG': return node.getAttribute('alt') ? '![' + node.getAttribute('alt') + '](' + (node.getAttribute('src') || '') + ')' : '';
      case 'BLOCKQUOTE':
        return '\n\n' + inline(node).trim().split('\n').map(function (l) { return '> ' + l; }).join('\n') + '\n\n';
      case 'UL': case 'OL': {
        var items = [], n = 0;
        for (var c = node.firstElementChild; c; c = c.nextElementSibling) {
          if (c.tagName.toUpperCase() !== 'LI') continue;
          n++;
          var bullet = tag === 'OL' ? n + '. ' : '- ';
          var body = inline(c).trim().replace(/\n{3,}/g, '\n\n').split('\n').join('\n' + new Array(bullet.length + 1).join(' '));
          items.push(bullet + body);
        }
        return '\n\n' + items.join('\n') + '\n\n';
      }
      case 'TABLE': return tableToMd(node);
      default: return inline(node);
    }
  }

  function toMarkdown(el) {
    try {
      return md(el, 0).replace(/[ \t]+\n/g, '\n').replace(/\n{3,}/g, '\n\n').trim();
    } catch (e) {
      return (el.innerText || '').trim();
    }
  }

  // ---- operations --------------------------------------------------------
  // A visible button/link whose whole text is "Log in" / "Sign up" ...: only
  // signed-out visitors see these.
  function visibleLoginText() {
    var rx = re(cfg.loginText);
    if (!rx) return false;
    var els = document.querySelectorAll('button, a, [role="button"]');
    for (var i = 0; i < els.length; i++) {
      var t = (els[i].innerText || '').trim();
      if (t && t.length < 40 && rx.test(t) && visible(els[i])) return true;
    }
    return false;
  }

  function probe() {
    var titleRe = re(cfg.challengeTitle);
    var challenge = (titleRe && titleRe.test(document.title || '')) || anyVisible(cfg.challengeMarkers);
    var path = location.pathname || '';
    var onLoginPath = (cfg.loginPaths || []).some(function (p) { return path.indexOf(p) >= 0; });
    var login = onLoginPath || anyVisible(cfg.loginMarkers) || visibleLoginText();
    var composer = pickVisible(cfg.composer);
    var state = 'unknown';
    if (challenge) state = 'challenge';
    else if (login) state = 'login';
    else if (composer) state = 'ready';
    return { state: state, host: location.hostname, url: location.href, composer: composer ? composer.sel : '' };
  }

  function snapshot() {
    var a = allMatches(cfg.assistant);
    var stop = pickVisible(cfg.stop);
    var count = a ? a.list.length : 0;
    return {
      count: count,
      last: count ? toMarkdown(a.list[count - 1]) : '',
      generating: !!stop,
      matched: a ? a.sel : '',
      url: location.href
    };
  }

  function fill(text) {
    var c = pickVisible(cfg.composer);
    if (!c) return { ok: false, reason: 'no composer' };
    var el = c.el;
    el.focus();
    var tag = el.tagName.toUpperCase();
    if (tag === 'TEXTAREA' || tag === 'INPUT') {
      var setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(el), 'value').set;
      setter.call(el, text);
      el.dispatchEvent(new Event('input', { bubbles: true }));
      return { ok: true, len: el.value.length, sel: c.sel };
    }
    // contenteditable (ProseMirror and friends): select everything, then
    // deliver the text as a paste, which editors parse into paragraphs
    // properly; fall back to insertText if the editor ignored the paste.
    var sel = window.getSelection();
    var range = document.createRange();
    range.selectNodeContents(el);
    sel.removeAllRanges();
    sel.addRange(range);
    var pasted = false;
    try {
      var dt = new DataTransfer();
      dt.setData('text/plain', text);
      pasted = !el.dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true }));
    } catch (e) { pasted = false; }
    if (!pasted || (el.innerText || '').trim().length === 0) {
      document.execCommand('selectAll', false, null);
      document.execCommand('insertText', false, text);
    }
    return { ok: true, len: (el.innerText || '').length, sel: c.sel };
  }

  function send() {
    var b = pickVisible(cfg.send);
    if (!b) return { clicked: false };
    if (b.el.disabled || b.el.getAttribute('aria-disabled') === 'true') return { clicked: false, disabled: true };
    b.el.click();
    return { clicked: true, sel: b.sel };
  }

  function stop() {
    var b = pickVisible(cfg.stop);
    if (b) b.el.click();
    return { stopped: !!b };
  }

  // Why did no reply start? Looks at what the page says, only when asked.
  function diagnose() {
    var text = '';
    try { text = (document.body.innerText || '').slice(-6000); } catch (e) {}
    var q = re(cfg.quotaText);
    var m = q ? q.exec(text) : null;
    var line = '';
    if (m) {
      var at = m.index;
      var start = text.lastIndexOf('\n', at) + 1;
      var end = text.indexOf('\n', at);
      line = text.slice(start, end < 0 ? text.length : end).trim().slice(0, 200);
    }
    return { quota: !!m, quotaLine: line, probe: probe() };
  }

  try {
    var r;
    switch (op) {
      case 'probe': r = probe(); break;
      case 'snapshot': r = snapshot(); break;
      case 'fill': r = fill(String(arg)); break;
      case 'send': r = send(); break;
      case 'stop': r = stop(); break;
      case 'diagnose': r = diagnose(); break;
      default: r = { error: 'unknown op' };
    }
    return JSON.stringify(r);
  } catch (e) {
    return JSON.stringify({ error: String(e && e.message || e) });
  }
})
