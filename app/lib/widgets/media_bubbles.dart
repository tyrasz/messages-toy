import 'package:flutter/material.dart';
import '../models/message.dart';

/// Widget for displaying image messages
class ImageMessageBubble extends StatelessWidget {
  final Message message;
  final bool isMe;

  const ImageMessageBubble({
    super.key,
    required this.message,
    required this.isMe,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => _openFullScreen(context),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(12),
        child: ConstrainedBox(
          constraints: const BoxConstraints(
            maxWidth: 250,
            maxHeight: 300,
          ),
          child: message.mediaUrl != null
              ? Image.network(
                  message.mediaUrl!,
                  fit: BoxFit.cover,
                  loadingBuilder: (context, child, loadingProgress) {
                    if (loadingProgress == null) return child;
                    return Container(
                      width: 200,
                      height: 150,
                      color: Colors.grey[300],
                      child: Center(
                        child: CircularProgressIndicator(
                          value: loadingProgress.expectedTotalBytes != null
                              ? loadingProgress.cumulativeBytesLoaded /
                                  loadingProgress.expectedTotalBytes!
                              : null,
                        ),
                      ),
                    );
                  },
                  errorBuilder: (context, error, stackTrace) {
                    return Container(
                      width: 200,
                      height: 150,
                      color: Colors.grey[300],
                      child: const Icon(Icons.broken_image, size: 48),
                    );
                  },
                )
              : Container(
                  width: 200,
                  height: 150,
                  color: Colors.grey[300],
                  child: const Center(
                    child: CircularProgressIndicator(),
                  ),
                ),
        ),
      ),
    );
  }

  void _openFullScreen(BuildContext context) {
    if (message.mediaUrl == null) return;
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (context) => _FullScreenImage(imageUrl: message.mediaUrl!),
      ),
    );
  }
}

class _FullScreenImage extends StatelessWidget {
  final String imageUrl;

  const _FullScreenImage({required this.imageUrl});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.transparent,
        iconTheme: const IconThemeData(color: Colors.white),
      ),
      body: Center(
        child: InteractiveViewer(
          child: Image.network(imageUrl),
        ),
      ),
    );
  }
}

/// Widget for displaying video messages with thumbnail
class VideoMessageBubble extends StatelessWidget {
  final Message message;
  final bool isMe;

  const VideoMessageBubble({
    super.key,
    required this.message,
    required this.isMe,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => _openVideoPlayer(context),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(12),
        child: Stack(
          alignment: Alignment.center,
          children: [
            // Thumbnail or placeholder
            Container(
              width: 250,
              height: 180,
              color: Colors.grey[800],
              child: message.thumbnailUrl != null
                  ? Image.network(
                      message.thumbnailUrl!,
                      fit: BoxFit.cover,
                      errorBuilder: (context, error, stackTrace) {
                        return _buildPlaceholder();
                      },
                    )
                  : _buildPlaceholder(),
            ),
            // Play button overlay
            Container(
              width: 56,
              height: 56,
              decoration: BoxDecoration(
                color: Colors.black54,
                shape: BoxShape.circle,
                border: Border.all(color: Colors.white, width: 2),
              ),
              child: const Icon(
                Icons.play_arrow,
                color: Colors.white,
                size: 36,
              ),
            ),
            // Duration badge
            if (message.mediaDuration != null)
              Positioned(
                bottom: 8,
                right: 8,
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 4,
                  ),
                  decoration: BoxDecoration(
                    color: Colors.black54,
                    borderRadius: BorderRadius.circular(4),
                  ),
                  child: Text(
                    _formatDuration(message.mediaDuration!),
                    style: const TextStyle(
                      color: Colors.white,
                      fontSize: 12,
                    ),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildPlaceholder() {
    return const Center(
      child: Icon(
        Icons.videocam,
        color: Colors.white54,
        size: 48,
      ),
    );
  }

  String _formatDuration(int seconds) {
    final minutes = seconds ~/ 60;
    final secs = seconds % 60;
    return '${minutes.toString().padLeft(2, '0')}:${secs.toString().padLeft(2, '0')}';
  }

  void _openVideoPlayer(BuildContext context) {
    if (message.mediaUrl == null) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Video not available')),
      );
      return;
    }
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (context) => VideoPlayerScreen(
          videoUrl: message.mediaUrl!,
          thumbnailUrl: message.thumbnailUrl,
        ),
      ),
    );
  }
}

/// Full-screen video player
class VideoPlayerScreen extends StatefulWidget {
  final String videoUrl;
  final String? thumbnailUrl;

  const VideoPlayerScreen({
    super.key,
    required this.videoUrl,
    this.thumbnailUrl,
  });

  @override
  State<VideoPlayerScreen> createState() => _VideoPlayerScreenState();
}

class _VideoPlayerScreenState extends State<VideoPlayerScreen> {
  // Note: In production, use video_player package
  // This is a placeholder that shows the video URL
  // Real implementation would use VideoPlayerController
  bool _isPlaying = false;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.transparent,
        iconTheme: const IconThemeData(color: Colors.white),
        title: const Text('Video', style: TextStyle(color: Colors.white)),
      ),
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            // Placeholder for video player
            // In production, replace with actual video_player widget
            Container(
              width: double.infinity,
              height: 250,
              color: Colors.grey[900],
              child: Stack(
                alignment: Alignment.center,
                children: [
                  if (widget.thumbnailUrl != null)
                    Image.network(
                      widget.thumbnailUrl!,
                      fit: BoxFit.contain,
                      errorBuilder: (_, __, ___) => const SizedBox(),
                    ),
                  IconButton(
                    onPressed: () {
                      setState(() => _isPlaying = !_isPlaying);
                      // TODO: Implement actual video playback
                      ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(
                          content: Text('Video playback requires video_player package'),
                        ),
                      );
                    },
                    icon: Icon(
                      _isPlaying ? Icons.pause_circle : Icons.play_circle,
                      color: Colors.white,
                      size: 72,
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 16),
            Text(
              'Video URL: ${widget.videoUrl}',
              style: const TextStyle(color: Colors.white54, fontSize: 12),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }
}

/// Widget for displaying document messages
class DocumentMessageBubble extends StatelessWidget {
  final Message message;
  final bool isMe;

  const DocumentMessageBubble({
    super.key,
    required this.message,
    required this.isMe,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => _openDocument(context),
      child: Container(
        width: 220,
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: isMe
              ? Colors.white.withOpacity(0.15)
              : Colors.grey[300],
          borderRadius: BorderRadius.circular(12),
        ),
        child: Row(
          children: [
            // Document icon
            Container(
              width: 48,
              height: 48,
              decoration: BoxDecoration(
                color: _getDocumentColor(),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Icon(
                _getDocumentIcon(),
                color: Colors.white,
                size: 28,
              ),
            ),
            const SizedBox(width: 12),
            // Document info
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    _getDocumentName(),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontWeight: FontWeight.w500,
                      color: isMe ? Colors.white : Colors.black87,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Row(
                    children: [
                      Text(
                        _getDocumentType(),
                        style: TextStyle(
                          fontSize: 12,
                          color: isMe ? Colors.white70 : Colors.grey[600],
                        ),
                      ),
                      if (message.pageCount != null) ...[
                        Text(
                          ' • ${message.pageCount} pages',
                          style: TextStyle(
                            fontSize: 12,
                            color: isMe ? Colors.white70 : Colors.grey[600],
                          ),
                        ),
                      ],
                    ],
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  String _getDocumentName() {
    if (message.content != null && message.content!.isNotEmpty) {
      return message.content!;
    }
    final contentType = message.mediaContentType ?? '';
    if (contentType.contains('pdf')) return 'PDF Document';
    if (contentType.contains('word')) return 'Word Document';
    if (contentType.contains('excel') || contentType.contains('spreadsheet')) {
      return 'Spreadsheet';
    }
    return 'Document';
  }

  String _getDocumentType() {
    final contentType = message.mediaContentType ?? '';
    if (contentType.contains('pdf')) return 'PDF';
    if (contentType.contains('word')) return 'DOC';
    if (contentType.contains('excel') || contentType.contains('spreadsheet')) {
      return 'XLS';
    }
    if (contentType.contains('text/plain')) return 'TXT';
    return 'FILE';
  }

  Color _getDocumentColor() {
    final contentType = message.mediaContentType ?? '';
    if (contentType.contains('pdf')) return Colors.red[600]!;
    if (contentType.contains('word')) return Colors.blue[600]!;
    if (contentType.contains('excel') || contentType.contains('spreadsheet')) {
      return Colors.green[600]!;
    }
    return Colors.grey[600]!;
  }

  IconData _getDocumentIcon() {
    final contentType = message.mediaContentType ?? '';
    if (contentType.contains('pdf')) return Icons.picture_as_pdf;
    if (contentType.contains('word')) return Icons.description;
    if (contentType.contains('excel') || contentType.contains('spreadsheet')) {
      return Icons.table_chart;
    }
    return Icons.insert_drive_file;
  }

  void _openDocument(BuildContext context) {
    if (message.mediaUrl == null) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Document not available')),
      );
      return;
    }
    // TODO: Open document with url_launcher or in-app viewer
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('Opening: ${message.mediaUrl}')),
    );
  }
}

/// Widget for displaying audio/voice messages
class AudioMessageBubble extends StatefulWidget {
  final Message message;
  final bool isMe;

  const AudioMessageBubble({
    super.key,
    required this.message,
    required this.isMe,
  });

  @override
  State<AudioMessageBubble> createState() => _AudioMessageBubbleState();
}

class _AudioMessageBubbleState extends State<AudioMessageBubble> {
  bool _isPlaying = false;
  double _progress = 0.0;

  @override
  Widget build(BuildContext context) {
    final duration = widget.message.mediaDuration ?? 0;

    return Container(
      width: 220,
      padding: const EdgeInsets.all(8),
      decoration: BoxDecoration(
        color: widget.isMe
            ? Colors.white.withOpacity(0.15)
            : Colors.grey[300],
        borderRadius: BorderRadius.circular(20),
      ),
      child: Row(
        children: [
          // Play/pause button
          GestureDetector(
            onTap: _togglePlayback,
            child: Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: widget.isMe
                    ? Colors.white.withOpacity(0.3)
                    : Theme.of(context).primaryColor,
                shape: BoxShape.circle,
              ),
              child: Icon(
                _isPlaying ? Icons.pause : Icons.play_arrow,
                color: widget.isMe ? Colors.white : Colors.white,
                size: 24,
              ),
            ),
          ),
          const SizedBox(width: 8),
          // Waveform and progress
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Waveform visualization (simplified bars)
                SizedBox(
                  height: 24,
                  child: CustomPaint(
                    painter: _WaveformPainter(
                      progress: _progress,
                      isMe: widget.isMe,
                      color: widget.isMe ? Colors.white : Theme.of(context).primaryColor,
                    ),
                    size: const Size(double.infinity, 24),
                  ),
                ),
                const SizedBox(height: 4),
                // Duration
                Text(
                  _formatDuration((_progress * duration).round(), duration),
                  style: TextStyle(
                    fontSize: 11,
                    color: widget.isMe ? Colors.white70 : Colors.grey[600],
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  void _togglePlayback() {
    setState(() {
      _isPlaying = !_isPlaying;
      if (_isPlaying) {
        // Simulate playback progress
        _simulatePlayback();
      }
    });
    // TODO: Implement actual audio playback with just_audio or audioplayers
  }

  void _simulatePlayback() {
    final duration = widget.message.mediaDuration ?? 10;
    final interval = Duration(milliseconds: (duration * 10).clamp(50, 100));

    Future.doWhile(() async {
      if (!_isPlaying || !mounted) return false;
      await Future.delayed(interval);
      if (!mounted) return false;

      setState(() {
        _progress += 0.01;
        if (_progress >= 1.0) {
          _progress = 0.0;
          _isPlaying = false;
        }
      });
      return _isPlaying && _progress < 1.0;
    });
  }

  String _formatDuration(int current, int total) {
    String format(int seconds) {
      final mins = seconds ~/ 60;
      final secs = seconds % 60;
      return '${mins.toString().padLeft(1, '0')}:${secs.toString().padLeft(2, '0')}';
    }
    return '${format(current)} / ${format(total)}';
  }
}

/// Custom painter for audio waveform visualization
class _WaveformPainter extends CustomPainter {
  final double progress;
  final bool isMe;
  final Color color;

  _WaveformPainter({
    required this.progress,
    required this.isMe,
    required this.color,
  });

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..strokeWidth = 3
      ..strokeCap = StrokeCap.round;

    // Generate pseudo-random bar heights (in production, use actual waveform data)
    final barCount = 25;
    final barWidth = size.width / barCount;
    final heights = List.generate(barCount, (i) {
      // Create a wave-like pattern
      final base = 0.3 + 0.7 * ((i % 5) / 5);
      return base * size.height;
    });

    for (int i = 0; i < barCount; i++) {
      final x = i * barWidth + barWidth / 2;
      final height = heights[i];
      final y1 = (size.height - height) / 2;
      final y2 = y1 + height;

      // Color based on progress
      final isPlayed = i / barCount <= progress;
      paint.color = isPlayed
          ? color
          : color.withOpacity(0.3);

      canvas.drawLine(
        Offset(x, y1),
        Offset(x, y2),
        paint,
      );
    }
  }

  @override
  bool shouldRepaint(_WaveformPainter oldDelegate) {
    return oldDelegate.progress != progress;
  }
}
