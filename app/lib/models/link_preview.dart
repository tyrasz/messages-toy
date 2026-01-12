class LinkPreview {
  final String id;
  final String url;
  final String? title;
  final String? description;
  final String? imageUrl;
  final String? siteName;
  final String? faviconUrl;

  LinkPreview({
    required this.id,
    required this.url,
    this.title,
    this.description,
    this.imageUrl,
    this.siteName,
    this.faviconUrl,
  });

  factory LinkPreview.fromJson(Map<String, dynamic> json) {
    return LinkPreview(
      id: json['id'] as String? ?? '',
      url: json['url'] as String,
      title: json['title'] as String?,
      description: json['description'] as String?,
      imageUrl: json['image_url'] as String?,
      siteName: json['site_name'] as String?,
      faviconUrl: json['favicon_url'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'url': url,
      if (title != null) 'title': title,
      if (description != null) 'description': description,
      if (imageUrl != null) 'image_url': imageUrl,
      if (siteName != null) 'site_name': siteName,
      if (faviconUrl != null) 'favicon_url': faviconUrl,
    };
  }

  bool get hasContent => title != null || description != null;
  bool get hasImage => imageUrl != null && imageUrl!.isNotEmpty;
}

// Utility to extract URLs from text
class UrlExtractor {
  static final _urlRegex = RegExp(
    r'https?://[^\s<>\[\]"{}|\\^`]+',
    caseSensitive: false,
  );

  static List<String> extractUrls(String text) {
    final matches = _urlRegex.allMatches(text);
    return matches.map((m) => m.group(0)!).toList();
  }

  static bool hasUrl(String text) {
    return _urlRegex.hasMatch(text);
  }

  static String? getFirstUrl(String text) {
    final match = _urlRegex.firstMatch(text);
    return match?.group(0);
  }
}
