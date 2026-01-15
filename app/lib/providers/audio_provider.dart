import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../services/audio_service.dart';

/// Provider for the shared AudioService instance
final audioServiceProvider = Provider<AudioService>((ref) {
  final service = AudioService();
  ref.onDispose(() => service.dispose());
  return service;
});

/// Stream provider for recording state
final recordingStateProvider = StreamProvider<RecordingState>((ref) {
  final service = ref.watch(audioServiceProvider);
  return service.recordingStateStream;
});

/// Stream provider for recording duration
final recordingDurationProvider = StreamProvider<Duration>((ref) {
  final service = ref.watch(audioServiceProvider);
  return service.recordingDurationStream;
});

/// Stream provider for recording amplitude
final recordingAmplitudeProvider = StreamProvider<double>((ref) {
  final service = ref.watch(audioServiceProvider);
  return service.recordingAmplitudeStream;
});

/// Stream provider for playback state
final playbackStateProvider = StreamProvider<PlaybackState>((ref) {
  final service = ref.watch(audioServiceProvider);
  return service.playbackStateStream;
});

/// Stream provider for playback position
final playbackPositionProvider = StreamProvider<Duration>((ref) {
  final service = ref.watch(audioServiceProvider);
  return service.playbackPositionStream;
});

/// Stream provider for playback duration
final playbackDurationProvider = StreamProvider<Duration>((ref) {
  final service = ref.watch(audioServiceProvider);
  return service.playbackDurationStream;
});
