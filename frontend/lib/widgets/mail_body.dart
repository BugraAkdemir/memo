import 'package:flutter/material.dart';
import 'package:flutter_html/flutter_html.dart';
import 'package:flutter_html_table/flutter_html_table.dart';
import 'package:html/dom.dart' as dom;
import 'package:html/parser.dart' as parser;
import 'package:url_launcher/url_launcher.dart';

import '../core/l10n.dart';

// Static Flutter rendering only: no browser, scripts, remote CSS or image fetches.
// Keep layout/typography and embedded CSS; links require an explicit user tap.
dom.Document prepareMailHtml(String source) {
  final document = parser.parse(source);
  const allowed = {
    'html',
    'head',
    'body',
    'style',
    'div',
    'span',
    'p',
    'br',
    'hr',
    'h1',
    'h2',
    'h3',
    'h4',
    'h5',
    'h6',
    'b',
    'strong',
    'i',
    'em',
    'u',
    's',
    'del',
    'small',
    'sub',
    'sup',
    'pre',
    'code',
    'blockquote',
    'ul',
    'ol',
    'li',
    'dl',
    'dt',
    'dd',
    'a',
    'table',
    'thead',
    'tbody',
    'tfoot',
    'tr',
    'th',
    'td',
    'caption',
    'font',
    'center',
  };
  const discard = {
    'script',
    'iframe',
    'object',
    'embed',
    'svg',
    'math',
    'video',
    'audio',
    'source',
    'link',
    'meta',
    'base',
    'input',
    'button',
    'textarea',
    'select',
    'template',
    'noscript',
  };
  const attributes = {
    'id',
    'class',
    'style',
    'href',
    'title',
    'dir',
    'lang',
    'align',
    'width',
    'height',
    'colspan',
    'rowspan',
    'border',
    'cellpadding',
    'cellspacing',
    'bgcolor',
    'color',
    'size',
    'face',
    'start',
    'type',
  };
  for (final element in document.querySelectorAll('*').toList()) {
    final tag = element.localName;
    if (tag == 'img') {
      element.replaceWith(dom.Text(element.attributes['alt'] ?? ''));
    } else if (discard.contains(tag)) {
      element.remove();
    } else if (!allowed.contains(tag)) {
      element.replaceWith(
        dom.Element.tag('div')..nodes.addAll(element.nodes.toList()),
      );
    } else {
      element.attributes.removeWhere((key, value) => !attributes.contains(key));
      final href = element.attributes['href'];
      if (href != null && safeMailLink(href) == null) {
        element.attributes.remove('href');
      }
    }
  }
  return document;
}

Uri? safeMailLink(String value) {
  final uri = Uri.tryParse(value.trim());
  if (uri == null || !{'https', 'http', 'mailto'}.contains(uri.scheme)) {
    return null;
  }
  if (uri.scheme != 'mailto' && uri.host.isEmpty) return null;
  return uri;
}

class MailBody extends StatefulWidget {
  const MailBody({super.key, required this.textBody, required this.htmlBody});

  final String textBody;
  final String htmlBody;

  @override
  State<MailBody> createState() => _MailBodyState();
}

class _MailBodyState extends State<MailBody> {
  bool _plain = false;
  dom.Document? _document;

  @override
  void didUpdateWidget(covariant MailBody oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.htmlBody != widget.htmlBody) _document = null;
  }

  @override
  Widget build(BuildContext context) {
    if (widget.htmlBody.trim().isEmpty) {
      return SelectableText(widget.textBody);
    }
    final document = _document ??= prepareMailHtml(widget.htmlBody);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            TextButton(
              onPressed: () => setState(() => _plain = !_plain),
              child: Text(
                L10n.t(_plain ? 'mail_body_formatted' : 'mail_body_plain'),
              ),
            ),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                L10n.t('mail_body_remote_blocked'),
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ),
          ],
        ),
        if (_plain)
          SelectableText(
            widget.textBody.isNotEmpty
                ? widget.textBody
                : document.body?.text ?? '',
          )
        else
          ColoredBox(
            color: Colors.white,
            child: SelectionArea(
              child: Html.fromDom(
                document: document,
                extensions: [TableHtmlExtension()],
                doNotRenderTheseTags: const {
                  'img',
                  'script',
                  'iframe',
                  'object',
                  'embed',
                },
                style: {
                  'html': Style(color: Colors.black, fontSize: FontSize(14)),
                },
                onLinkTap: (url, attributes, element) async {
                  final uri = safeMailLink(url ?? '');
                  if (uri != null) {
                    await launchUrl(uri, mode: LaunchMode.externalApplication);
                  }
                },
              ),
            ),
          ),
      ],
    );
  }
}
