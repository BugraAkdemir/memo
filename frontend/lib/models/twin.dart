class TwinAlias {
  final String channel;
  final String address;
  const TwinAlias(this.channel, this.address);
  factory TwinAlias.fromJson(Map<String, dynamic> j) =>
      TwinAlias(j['channel'] as String, j['address'] as String);
  Map<String, dynamic> toJson() => {'channel': channel, 'address': address};
}

class TwinContact {
  final String id;
  final String name;
  final String policy;
  final List<TwinAlias> aliases;
  final Map<String, dynamic> style;
  const TwinContact(this.id, this.name, this.policy, this.aliases, this.style);
  factory TwinContact.fromJson(Map<String, dynamic> j) => TwinContact(
    j['id'] as String,
    j['name'] as String,
    j['autonomy_policy'] as String,
    (j['aliases'] as List)
        .map((a) => TwinAlias.fromJson(Map<String, dynamic>.from(a as Map)))
        .toList(),
    Map<String, dynamic>.from(j['style'] as Map),
  );
}

class TwinState {
  final List<TwinContact> contacts;
  final Map<String, dynamic> preferences;
  final List<Map<String, dynamic>> projects;
  const TwinState(this.contacts, this.preferences, this.projects);
  factory TwinState.fromJson(Map<String, dynamic> j) => TwinState(
    (j['contacts'] as List)
        .map((c) => TwinContact.fromJson(Map<String, dynamic>.from(c as Map)))
        .toList(),
    Map<String, dynamic>.from(j['preferences'] as Map),
    (j['projects'] as List)
        .map((p) => Map<String, dynamic>.from(p as Map))
        .toList(),
  );
}
