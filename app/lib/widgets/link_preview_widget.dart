import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';
import '../models/link_preview.dart';
import '../providers/auth_provider.dart';

// Cache for link previews
final _linkPreviewCache = <String, LinkPreview>{};
final _linkPreviewLoading = <String>{};

class LinkPreviewWidget extends ConsumerStatefulWidget {
  final String url;
  final bool isMe;

  const LinkPreviewWidget({
    super.key,
    required this.url,
    required this.isMe,
  });

  @override
  ConsumerState<LinkPreviewWidget> createState() => _LinkPreviewWidgetState();
}

class _LinkPreviewWidgetState extends ConsumerState<LinkPreviewWidget> {
  LinkPreview? _preview;
  bool _isLoading = false;
  bool _hasError = false;

  @override
  void initState() {
    super.initState();
    _loadPreview();
  }

  Future<void> _loadPreview() async {
    // Check cache first
    if (_linkPreviewCache.containsKey(widget.url)) {
      setState(() {
        _preview = _linkPreviewCache[widget.url];
      });
      return;
    }

    // Avoid duplicate loading
    if (_linkPreviewLoading.contains(widget.url)) {
      // Wait for another widget to finish loading
      await Future.delayed(const Duration(milliseconds: 500));
      if (_linkPreviewCache.containsKey(widget.url)) {
        setState(() {
          _preview = _linkPreviewCache[widget.url];
        });
      }
      return;
    }

    setState(() {
      _isLoading = true;
    });

    _linkPreviewLoading.add(widget.url);

    try {
      final apiService = ref.read(apiServiceProvider);
      final preview = await apiService.fetchLinkPreview(widget.url);

      if (preview != null && preview.hasContent) {
        _linkPreviewCache[widget.url] = preview;
        if (mounted) {
          setState(() {
            _preview = preview;
            _isLoading = false;
          });
        }
      } else {
        if (mounted) {
          setState(() {
            _hasError = true;
            _isLoading = false;
          });
        }
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _hasError = true;
          _isLoading = false;
        });
      }
    } finally {
      _linkPreviewLoading.remove(widget.url);
    }
  }

  Future<void> _launchUrl() async {
    final uri = Uri.parse(widget.url);
    if (await canLaunchUrl(uri)) {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_isLoading) {
      return Container(
        margin: const EdgeInsets.only(top: 8),
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: widget.isMe
              ? Colors.white.withOpacity(0.15)
              : Colors.grey[100],
          borderRadius: BorderRadius.circular(8),
        ),
        child: Row(
          children: [
            SizedBox(
              width: 16,
              height: 16,
              child: CircularProgressIndicator(
                strokeWidth: 2,
                valueColor: AlwaysStoppedAnimation(
                  widget.isMe ? Colors.white70 : Colors.grey[400],
                ),
              ),
            ),
            const SizedBox(width: 8),
            Text(
              'Loading preview...',
              style: TextStyle(
                fontSize: 12,
                color: widget.isMe ? Colors.white60 : Colors.grey[500],
              ),
            ),
          ],
        ),
      );
    }

    if (_hasError || _preview == null || !_preview!.hasContent) {
      return const SizedBox.shrink();
    }

    return GestureDetector(
      onTap: _launchUrl,
      child: Container(
        margin: const EdgeInsets.only(top: 8),
        decoration: BoxDecoration(
          color: widget.isMe
              ? Colors.white.withOpacity(0.15)
              : Colors.grey[100],
          borderRadius: BorderRadius.circular(8),
          border: Border.all(
            color: widget.isMe
                ? Colors.white.withOpacity(0.2)
                : Colors.grey[300]!,
          ),
        ),
        clipBehavior: Clip.antiAlias,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Preview image
            if (_preview!.hasImage)
              ClipRRect(
                borderRadius: const BorderRadius.only(
                  topLeft: Radius.circular(7),
                  topRight: Radius.circular(7),
                ),
                child: Image.network(
                  _preview!.imageUrl!,
                  width: double.infinity,
                  height: 120,
                  fit: BoxFit.cover,
                  errorBuilder: (context, error, stackTrace) =>
                      const SizedBox.shrink(),
                ),
              ),

            // Content
            Padding(
              padding: const EdgeInsets.all(10),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Site name
                  if (_preview!.siteName != null &&
                      _preview!.siteName!.isNotEmpty)
                    Row(
                      children: [
                        if (_preview!.faviconUrl != null)
                          Padding(
                            padding: const EdgeInsets.only(right: 6),
                            child: Image.network(
                              _preview!.faviconUrl!,
                              width: 14,
                              height: 14,
                              errorBuilder: (_, __, ___) =>
                                  const SizedBox.shrink(),
                            ),
                          ),
                        Text(
                          _preview!.siteName!.toUpperCase(),
                          style: TextStyle(
                            fontSize: 10,
                            fontWeight: FontWeight.bold,
                            letterSpacing: 0.5,
                            color: widget.isMe ? Colors.white60 : Colors.grey[500],
                          ),
                        ),
                      ],
                    ),
                  if (_preview!.siteName != null)
                    const SizedBox(height: 4),

                  // Title
                  if (_preview!.title != null && _preview!.title!.isNotEmpty)
                    Text(
                      _preview!.title!,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                        color: widget.isMe ? Colors.white : Colors.black87,
                      ),
                    ),
                  if (_preview!.title != null)
                    const SizedBox(height: 4),

                  // Description
                  if (_preview!.description != null &&
                      _preview!.description!.isNotEmpty)
                    Text(
                      _preview!.description!,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 12,
                        color: widget.isMe ? Colors.white70 : Colors.grey[600],
                      ),
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
