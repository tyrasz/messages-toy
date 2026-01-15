import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../providers/audio_provider.dart';
import '../services/audio_service.dart';

class VoiceRecorderWidget extends ConsumerStatefulWidget {
  final Function(String path)? onRecordingComplete;
  final VoidCallback? onCancel;

  const VoiceRecorderWidget({
    super.key,
    this.onRecordingComplete,
    this.onCancel,
  });

  @override
  ConsumerState<VoiceRecorderWidget> createState() => _VoiceRecorderWidgetState();
}

class _VoiceRecorderWidgetState extends ConsumerState<VoiceRecorderWidget>
    with SingleTickerProviderStateMixin {
  late AnimationController _pulseController;
  bool _isRecording = false;

  @override
  void initState() {
    super.initState();
    _pulseController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1000),
    );
  }

  @override
  void dispose() {
    _pulseController.dispose();
    super.dispose();
  }

  Future<void> _startRecording() async {
    final audioService = ref.read(audioServiceProvider);

    if (!await audioService.hasPermission()) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Microphone permission required')),
        );
      }
      return;
    }

    await audioService.startRecording();
    setState(() => _isRecording = true);
    _pulseController.repeat(reverse: true);
  }

  Future<void> _stopRecording() async {
    final audioService = ref.read(audioServiceProvider);
    final path = await audioService.stopRecording();
    _pulseController.stop();
    setState(() => _isRecording = false);

    if (path != null && widget.onRecordingComplete != null) {
      widget.onRecordingComplete!(path);
    }
  }

  Future<void> _cancelRecording() async {
    final audioService = ref.read(audioServiceProvider);
    await audioService.cancelRecording();
    _pulseController.stop();
    setState(() => _isRecording = false);
    widget.onCancel?.call();
  }

  String _formatDuration(Duration duration) {
    final minutes = duration.inMinutes.remainder(60).toString().padLeft(2, '0');
    final seconds = duration.inSeconds.remainder(60).toString().padLeft(2, '0');
    return '$minutes:$seconds';
  }

  @override
  Widget build(BuildContext context) {
    if (!_isRecording) {
      return IconButton(
        onPressed: _startRecording,
        icon: const Icon(Icons.mic),
        tooltip: 'Record voice message',
      );
    }

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          // Cancel button
          IconButton(
            onPressed: _cancelRecording,
            icon: const Icon(Icons.delete_outline, color: Colors.red),
            tooltip: 'Cancel',
          ),

          // Recording indicator and duration
          Expanded(
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              decoration: BoxDecoration(
                color: Colors.red.withOpacity(0.1),
                borderRadius: BorderRadius.circular(20),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  // Pulsing red dot
                  AnimatedBuilder(
                    animation: _pulseController,
                    builder: (context, child) {
                      return Container(
                        width: 12,
                        height: 12,
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          color: Colors.red.withOpacity(
                            0.5 + _pulseController.value * 0.5,
                          ),
                        ),
                      );
                    },
                  ),
                  const SizedBox(width: 8),

                  // Duration
                  Consumer(
                    builder: (context, ref, _) {
                      final durationAsync = ref.watch(recordingDurationProvider);
                      final duration = durationAsync.valueOrNull ?? Duration.zero;
                      return Text(
                        _formatDuration(duration),
                        style: const TextStyle(
                          fontWeight: FontWeight.w500,
                          fontSize: 14,
                        ),
                      );
                    },
                  ),

                  const SizedBox(width: 8),

                  // Amplitude visualization
                  Consumer(
                    builder: (context, ref, _) {
                      final amplitudeAsync = ref.watch(recordingAmplitudeProvider);
                      final amplitude = amplitudeAsync.valueOrNull ?? 0.0;
                      return SizedBox(
                        width: 60,
                        height: 24,
                        child: CustomPaint(
                          painter: _AmplitudePainter(amplitude: amplitude),
                        ),
                      );
                    },
                  ),
                ],
              ),
            ),
          ),

          // Send button
          IconButton(
            onPressed: _stopRecording,
            icon: Icon(
              Icons.send,
              color: Theme.of(context).primaryColor,
            ),
            tooltip: 'Send',
          ),
        ],
      ),
    );
  }
}

/// Paints audio amplitude visualization bars
class _AmplitudePainter extends CustomPainter {
  final double amplitude;
  static final List<double> _previousAmplitudes = List.filled(10, 0.0);
  static int _amplitudeIndex = 0;

  _AmplitudePainter({required this.amplitude}) {
    _previousAmplitudes[_amplitudeIndex] = amplitude;
    _amplitudeIndex = (_amplitudeIndex + 1) % _previousAmplitudes.length;
  }

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = Colors.red.withOpacity(0.6)
      ..strokeWidth = 3
      ..strokeCap = StrokeCap.round;

    final barCount = _previousAmplitudes.length;
    final barWidth = size.width / barCount;

    for (int i = 0; i < barCount; i++) {
      final idx = (_amplitudeIndex + i) % barCount;
      final amp = _previousAmplitudes[idx];
      final height = size.height * 0.3 + size.height * 0.7 * amp;
      final x = i * barWidth + barWidth / 2;
      final y1 = (size.height - height) / 2;
      final y2 = y1 + height;

      canvas.drawLine(Offset(x, y1), Offset(x, y2), paint);
    }
  }

  @override
  bool shouldRepaint(_AmplitudePainter oldDelegate) => true;
}

/// Compact voice recorder that shows in the message input area
class CompactVoiceRecorder extends ConsumerWidget {
  final Function(String path) onRecordingComplete;

  const CompactVoiceRecorder({
    super.key,
    required this.onRecordingComplete,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final recordingState = ref.watch(recordingStateProvider);

    return recordingState.when(
      data: (state) {
        if (state == RecordingState.recording) {
          return VoiceRecorderWidget(
            onRecordingComplete: onRecordingComplete,
          );
        }
        return const SizedBox.shrink();
      },
      loading: () => const SizedBox.shrink(),
      error: (_, __) => const SizedBox.shrink(),
    );
  }
}
