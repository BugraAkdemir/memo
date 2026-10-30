// Models for the Mail & Digital Twin subsystem (/api/mail/*).

class MailServerConfig {
  final String provider;
  final String imapHost;
  final int imapPort;
  final String smtpHost;
  final int smtpPort;
  final String authType;
  final String source;

  const MailServerConfig({
    this.provider = 'custom',
    this.imapHost = '',
    this.imapPort = 993,
    this.smtpHost = '',
    this.smtpPort = 587,
    this.authType = 'password',
    this.source = '',
  });

  factory MailServerConfig.fromJson(Map<String, dynamic> json) {
    return MailServerConfig(
      provider: json['provider'] as String? ?? 'custom',
      imapHost: json['imap_host'] as String? ?? '',
      imapPort: (json['imap_port'] as num?)?.toInt() ?? 993,
      smtpHost: json['smtp_host'] as String? ?? '',
      smtpPort: (json['smtp_port'] as num?)?.toInt() ?? 587,
      authType: json['auth_type'] as String? ?? 'password',
      source: json['source'] as String? ?? '',
    );
  }
}

class MailAccount {
  final String id;
  final String email;
  final String displayName;
  final String provider;
  final String authType;
  final MailServerConfig server;
  final int lastSeenUid;
  final String connectedAt;

  const MailAccount({
    required this.id,
    required this.email,
    this.displayName = '',
    this.provider = 'custom',
    this.authType = 'password',
    this.server = const MailServerConfig(),
    this.lastSeenUid = 0,
    this.connectedAt = '',
  });

  factory MailAccount.fromJson(Map<String, dynamic> json) {
    return MailAccount(
      id: json['id'] as String? ?? '',
      email: json['email'] as String? ?? '',
      displayName: json['display_name'] as String? ?? '',
      provider: json['provider'] as String? ?? 'custom',
      authType: json['auth_type'] as String? ?? 'password',
      server: json['server'] is Map
          ? MailServerConfig.fromJson(
              Map<String, dynamic>.from(json['server'] as Map),
            )
          : const MailServerConfig(),
      lastSeenUid: (json['last_seen_uid'] as num?)?.toInt() ?? 0,
      connectedAt: json['connected_at'] as String? ?? '',
    );
  }
}

class MailOAuthSession {
  final String state;
  final String provider;
  final String emailHint;
  final String authUrl;
  final String redirectUri;
  final String userCode;
  final String verifyUrl;
  final int intervalSec;
  final String status; // "pending", "completed", "error"
  final String error;
  final String accountId;
  final String email;

  const MailOAuthSession({
    required this.state,
    this.provider = '',
    this.emailHint = '',
    this.authUrl = '',
    this.redirectUri = '',
    this.userCode = '',
    this.verifyUrl = '',
    this.intervalSec = 0,
    this.status = 'pending',
    this.error = '',
    this.accountId = '',
    this.email = '',
  });

  factory MailOAuthSession.fromJson(Map<String, dynamic> json) {
    return MailOAuthSession(
      state: json['state'] as String? ?? '',
      provider: json['provider'] as String? ?? '',
      emailHint: json['email_hint'] as String? ?? '',
      authUrl: json['auth_url'] as String? ?? '',
      redirectUri: json['redirect_uri'] as String? ?? '',
      userCode: json['user_code'] as String? ?? '',
      verifyUrl: json['verify_url'] as String? ?? '',
      intervalSec: (json['interval_sec'] as num?)?.toInt() ?? 0,
      status: json['status'] as String? ?? 'pending',
      error: json['error'] as String? ?? '',
      accountId: json['account_id'] as String? ?? '',
      email: json['email'] as String? ?? '',
    );
  }
}

class MailMessageSummary {
  final int seqNum;
  final int uid;
  final String messageId;
  final String subject;
  final String from;
  final String to;
  final String date;
  final String snippet;
  final bool seen;
  final List<String> flags;
  final bool hasListUnsub;
  final bool isAutoSubmitted;
  final bool hasCalendarInvite;

  const MailMessageSummary({
    required this.seqNum,
    required this.uid,
    this.messageId = '',
    this.subject = '',
    this.from = '',
    this.to = '',
    this.date = '',
    this.snippet = '',
    this.seen = false,
    this.flags = const [],
    this.hasListUnsub = false,
    this.isAutoSubmitted = false,
    this.hasCalendarInvite = false,
  });

  factory MailMessageSummary.fromJson(Map<String, dynamic> json) {
    return MailMessageSummary(
      seqNum: (json['seq_num'] as num?)?.toInt() ?? 0,
      uid: (json['uid'] as num?)?.toInt() ?? 0,
      messageId: json['message_id'] as String? ?? '',
      subject: json['subject'] as String? ?? '',
      from: json['from'] as String? ?? '',
      to: json['to'] as String? ?? '',
      date: json['date'] as String? ?? '',
      snippet: json['snippet'] as String? ?? '',
      seen: json['seen'] as bool? ?? false,
      flags: json['flags'] is List
          ? (json['flags'] as List).map((e) => e.toString()).toList()
          : const [],
      hasListUnsub: json['has_list_unsub'] as bool? ?? false,
      isAutoSubmitted: json['is_auto_submitted'] as bool? ?? false,
      hasCalendarInvite: json['has_calendar_invite'] as bool? ?? false,
    );
  }
}

class MailAttachmentInfo {
  final String partId;
  final String filename;
  final String contentType;
  final int size;

  const MailAttachmentInfo({
    this.partId = '',
    this.filename = '',
    this.contentType = '',
    this.size = 0,
  });

  factory MailAttachmentInfo.fromJson(Map<String, dynamic> json) {
    return MailAttachmentInfo(
      partId: json['part_id'] as String? ?? '',
      filename: json['filename'] as String? ?? '',
      contentType: json['content_type'] as String? ?? '',
      size: (json['size'] as num?)?.toInt() ?? 0,
    );
  }
}

class MailCalendarInvite {
  final String uid;
  final String summary;
  final String description;
  final String location;
  final String organizer;
  final String startTime;
  final String endTime;
  final String method;

  const MailCalendarInvite({
    this.uid = '',
    this.summary = '',
    this.description = '',
    this.location = '',
    this.organizer = '',
    this.startTime = '',
    this.endTime = '',
    this.method = '',
  });

  factory MailCalendarInvite.fromJson(Map<String, dynamic> json) {
    return MailCalendarInvite(
      uid: json['uid'] as String? ?? '',
      summary: json['summary'] as String? ?? '',
      description: json['description'] as String? ?? '',
      location: json['location'] as String? ?? '',
      organizer: json['organizer'] as String? ?? '',
      startTime: json['start_time'] as String? ?? '',
      endTime: json['end_time'] as String? ?? '',
      method: json['method'] as String? ?? '',
    );
  }
}

class MailFlightInfo {
  final String reservationNumber;
  final String flightNumber;
  final String airline;
  final String departureAirport;
  final String arrivalAirport;
  final String departureTime;
  final String arrivalTime;

  const MailFlightInfo({
    this.reservationNumber = '',
    this.flightNumber = '',
    this.airline = '',
    this.departureAirport = '',
    this.arrivalAirport = '',
    this.departureTime = '',
    this.arrivalTime = '',
  });

  factory MailFlightInfo.fromJson(Map<String, dynamic> json) {
    return MailFlightInfo(
      reservationNumber: json['reservation_number'] as String? ?? '',
      flightNumber: json['flight_number'] as String? ?? '',
      airline: json['airline'] as String? ?? '',
      departureAirport: json['departure_airport'] as String? ?? '',
      arrivalAirport: json['arrival_airport'] as String? ?? '',
      departureTime: json['departure_time'] as String? ?? '',
      arrivalTime: json['arrival_time'] as String? ?? '',
    );
  }
}

class MailInvoiceInfo {
  final String providerName;
  final String invoiceId;
  final String totalAmount;
  final String currency;
  final String dueDate;
  final String paymentUrl;

  const MailInvoiceInfo({
    this.providerName = '',
    this.invoiceId = '',
    this.totalAmount = '',
    this.currency = '',
    this.dueDate = '',
    this.paymentUrl = '',
  });

  factory MailInvoiceInfo.fromJson(Map<String, dynamic> json) {
    return MailInvoiceInfo(
      providerName: json['provider_name'] as String? ?? '',
      invoiceId: json['invoice_id'] as String? ?? '',
      totalAmount: json['total_amount'] as String? ?? '',
      currency: json['currency'] as String? ?? '',
      dueDate: json['due_date'] as String? ?? '',
      paymentUrl: json['payment_url'] as String? ?? '',
    );
  }
}

class MailStructuredExtraction {
  final List<MailCalendarInvite> calendarInvites;
  final List<MailFlightInfo> flights;
  final List<MailInvoiceInfo> invoices;
  final String otpCode;

  const MailStructuredExtraction({
    this.calendarInvites = const [],
    this.flights = const [],
    this.invoices = const [],
    this.otpCode = '',
  });

  factory MailStructuredExtraction.fromJson(Map<String, dynamic> json) {
    return MailStructuredExtraction(
      calendarInvites: json['calendar_invites'] is List
          ? (json['calendar_invites'] as List)
                .whereType<Map>()
                .map((m) => MailCalendarInvite.fromJson(Map<String, dynamic>.from(m)))
                .toList()
          : const [],
      flights: json['flights'] is List
          ? (json['flights'] as List)
                .whereType<Map>()
                .map((m) => MailFlightInfo.fromJson(Map<String, dynamic>.from(m)))
                .toList()
          : const [],
      invoices: json['invoices'] is List
          ? (json['invoices'] as List)
                .whereType<Map>()
                .map((m) => MailInvoiceInfo.fromJson(Map<String, dynamic>.from(m)))
                .toList()
          : const [],
      otpCode: json['otp_code'] as String? ?? '',
    );
  }
}

class MailMessageDetail {
  final int seqNum;
  final int uid;
  final String messageId;
  final String inReplyTo;
  final String references;
  final String subject;
  final String from;
  final String to;
  final String cc;
  final String date;
  final String textBody;
  final String htmlBody;
  final List<MailAttachmentInfo> attachments;
  final MailStructuredExtraction structured;

  const MailMessageDetail({
    required this.seqNum,
    required this.uid,
    this.messageId = '',
    this.inReplyTo = '',
    this.references = '',
    this.subject = '',
    this.from = '',
    this.to = '',
    this.cc = '',
    this.date = '',
    this.textBody = '',
    this.htmlBody = '',
    this.attachments = const [],
    this.structured = const MailStructuredExtraction(),
  });

  factory MailMessageDetail.fromJson(Map<String, dynamic> json) {
    return MailMessageDetail(
      seqNum: (json['seq_num'] as num?)?.toInt() ?? 0,
      uid: (json['uid'] as num?)?.toInt() ?? 0,
      messageId: json['message_id'] as String? ?? '',
      inReplyTo: json['in_reply_to'] as String? ?? '',
      references: json['references'] as String? ?? '',
      subject: json['subject'] as String? ?? '',
      from: json['from'] as String? ?? '',
      to: json['to'] as String? ?? '',
      cc: json['cc'] as String? ?? '',
      date: json['date'] as String? ?? '',
      textBody: json['text_body'] as String? ?? '',
      htmlBody: json['html_body'] as String? ?? '',
      attachments: json['attachments'] is List
          ? (json['attachments'] as List)
                .whereType<Map>()
                .map((m) => MailAttachmentInfo.fromJson(Map<String, dynamic>.from(m)))
                .toList()
          : const [],
      structured: json['structured'] is Map
          ? MailStructuredExtraction.fromJson(
              Map<String, dynamic>.from(json['structured'] as Map),
            )
          : const MailStructuredExtraction(),
    );
  }
}

class MailWatcherStats {
  final int totalEvaluated;
  final int droppedMarketing;
  final int indexedInRag;
  final int passedToAutonomy;
  final int calendarExtracted;
  final int coldStarts;
  final String lastSyncAt;

  const MailWatcherStats({
    this.totalEvaluated = 0,
    this.droppedMarketing = 0,
    this.indexedInRag = 0,
    this.passedToAutonomy = 0,
    this.calendarExtracted = 0,
    this.coldStarts = 0,
    this.lastSyncAt = '',
  });

  factory MailWatcherStats.fromJson(Map<String, dynamic> json) {
    return MailWatcherStats(
      totalEvaluated: (json['total_evaluated'] as num?)?.toInt() ?? 0,
      droppedMarketing: (json['dropped_marketing'] as num?)?.toInt() ?? 0,
      indexedInRag: (json['indexed_in_rag'] as num?)?.toInt() ?? 0,
      passedToAutonomy: (json['passed_to_autonomy'] as num?)?.toInt() ?? 0,
      calendarExtracted: (json['calendar_extracted'] as num?)?.toInt() ?? 0,
      coldStarts: (json['cold_starts'] as num?)?.toInt() ?? 0,
      lastSyncAt: json['last_sync_at'] as String? ?? '',
    );
  }
}
