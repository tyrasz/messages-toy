import 'dart:async';
import 'dart:io';
import 'package:path_provider/path_provider.dart';
import 'package:record/record.dart';
import 'package:just_audio/just_audio.dart';

enum RecordingState { idle, recording, paused }
enum PlaybackState { idle, playing, paused }

class AudioService {
  final AudioRecorder _recorder = AudioRecorder();
  final AudioPlayer _player = AudioPlayer();

  RecordingState _recordingState = RecordingState.idle;
  PlaybackState _playbackState = PlaybackState.idle;

  String? _currentRecordingPath;
  String? _currentPlayingUrl;

  // Recording state
  final _recordingStateController = StreamController<RecordingState>.broadcast();
  final _recordingDurationController = StreamController<Duration>.broadcast();
  final _recordingAmplitudeController = StreamController<double>.broadcast();

  // Playback state
  final _playbackStateController = StreamController<PlaybackState>.broadcast();
  final _playbackPositionController = StreamController<Duration>.broadcast();
  final _playbackDurationController = StreamController<Duration>.broadcast();

  Timer? _durationTimer;
  Timer? _amplitudeTimer;
  DateTime? _recordingStartTime;

  // Getters
  RecordingState get recordingState => _recordingState;
  PlaybackState get playbackState => _playbackState;
  String? get currentRecordingPath => _currentRecordingPath;
  String? get currentPlayingUrl => _currentPlayingUrl;

  Stream<RecordingState> get recordingStateStream => _recordingStateController.stream;
  Stream<Duration> get recordingDurationStream => _recordingDurationController.stream;
  Stream<double> get recordingAmplitudeStream => _recordingAmplitudeController.stream;
  Stream<PlaybackState> get playbackStateStream => _playbackStateController.stream;
  Stream<Duration> get playbackPositionStream => _playbackPositionController.stream;
  Stream<Duration> get playbackDurationStream => _playbackDurationController.stream;

  AudioService() {
    // Listen to player state changes
    _player.playerStateStream.listen((state) {
      if (state.processingState == ProcessingState.completed) {
        _updatePlaybackState(PlaybackState.idle);
        _currentPlayingUrl = null;
      }
    });

    // Forward position updates
    _player.positionStream.listen((position) {
      _playbackPositionController.add(position);
    });

    // Forward duration updates
    _player.durationStream.listen((duration) {
      if (duration != null) {
        _playbackDurationController.add(duration);
      }
    });
  }

  /// Check if recording permission is granted
  Future<bool> hasPermission() async {
    return await _recorder.hasPermission();
  }

  /// Start recording audio
  Future<void> startRecording() async {
    if (_recordingState != RecordingState.idle) return;

    if (!await hasPermission()) {
      throw Exception('Microphone permission not granted');
    }

    // Get temp directory for recording
    final tempDir = await getTemporaryDirectory();
    final timestamp = DateTime.now().millisecondsSinceEpoch;
    _currentRecordingPath = '${tempDir.path}/voice_$timestamp.m4a';

    // Start recording with AAC codec
    await _recorder.start(
      const RecordConfig(
        encoder: AudioEncoder.aacLc,
        bitRate: 128000,
        sampleRate: 44100,
      ),
      path: _currentRecordingPath!,
    );

    _recordingStartTime = DateTime.now();
    _updateRecordingState(RecordingState.recording);

    // Start duration timer
    _durationTimer = Timer.periodic(const Duration(milliseconds: 100), (_) {
      if (_recordingStartTime != null) {
        final elapsed = DateTime.now().difference(_recordingStartTime!);
        _recordingDurationController.add(elapsed);
      }
    });

    // Start amplitude monitoring
    _amplitudeTimer = Timer.periodic(const Duration(milliseconds: 100), (_) async {
      final amplitude = await _recorder.getAmplitude();
      // Normalize amplitude to 0-1 range (amplitude.current is in dBFS, typically -160 to 0)
      final normalized = (amplitude.current + 60) / 60;
      _recordingAmplitudeController.add(normalized.clamp(0.0, 1.0));
    });
  }

  /// Stop recording and return the file path
  Future<String?> stopRecording() async {
    if (_recordingState != RecordingState.recording) return null;

    _durationTimer?.cancel();
    _amplitudeTimer?.cancel();

    final path = await _recorder.stop();
    _updateRecordingState(RecordingState.idle);
    _recordingStartTime = null;

    return path;
  }

  /// Cancel recording and delete the file
  Future<void> cancelRecording() async {
    if (_recordingState != RecordingState.recording) return;

    _durationTimer?.cancel();
    _amplitudeTimer?.cancel();

    await _recorder.stop();
    _updateRecordingState(RecordingState.idle);
    _recordingStartTime = null;

    // Delete the recording
    if (_currentRecordingPath != null) {
      final file = File(_currentRecordingPath!);
      if (await file.exists()) {
        await file.delete();
      }
    }
    _currentRecordingPath = null;
  }

  /// Play audio from URL
  Future<void> play(String url) async {
    // Stop any currently playing audio
    if (_playbackState == PlaybackState.playing && _currentPlayingUrl != url) {
      await stop();
    }

    if (_currentPlayingUrl == url && _playbackState == PlaybackState.paused) {
      // Resume playback
      await _player.play();
      _updatePlaybackState(PlaybackState.playing);
      return;
    }

    // Start new playback
    _currentPlayingUrl = url;
    await _player.setUrl(url);
    await _player.play();
    _updatePlaybackState(PlaybackState.playing);
  }

  /// Pause audio playback
  Future<void> pause() async {
    if (_playbackState != PlaybackState.playing) return;
    await _player.pause();
    _updatePlaybackState(PlaybackState.paused);
  }

  /// Stop audio playback
  Future<void> stop() async {
    await _player.stop();
    _updatePlaybackState(PlaybackState.idle);
    _currentPlayingUrl = null;
  }

  /// Seek to position
  Future<void> seek(Duration position) async {
    await _player.seek(position);
  }

  void _updateRecordingState(RecordingState state) {
    _recordingState = state;
    _recordingStateController.add(state);
  }

  void _updatePlaybackState(PlaybackState state) {
    _playbackState = state;
    _playbackStateController.add(state);
  }

  /// Clean up resources
  void dispose() {
    _durationTimer?.cancel();
    _amplitudeTimer?.cancel();
    _recorder.dispose();
    _player.dispose();
    _recordingStateController.close();
    _recordingDurationController.close();
    _recordingAmplitudeController.close();
    _playbackStateController.close();
    _playbackPositionController.close();
    _playbackDurationController.close();
  }
}
