import 'package:audio_service/audio_service.dart';
import 'package:audio_session/audio_session.dart';
import 'package:just_audio/just_audio.dart';

/// The background audio handler.
///
/// audio_service owns the platform side — the media notification, lock-screen
/// controls, Android Auto, headset buttons — and just_audio does the decoding.
/// Keeping them in one class means the notification can never disagree with
/// what is actually playing: every state change flows out of the same player.
class AtticAudioHandler extends BaseAudioHandler with QueueHandler, SeekHandler {
  AtticAudioHandler() {
    _player.playbackEventStream.listen(
      _broadcastState,
      onError: (Object error, StackTrace stack) {
        // A dead URL (server restarted, token expired) must not wedge the
        // notification in "playing".
        playbackState.add(playbackState.value.copyWith(
          processingState: AudioProcessingState.error,
          playing: false,
        ));
      },
    );

    // Keep the notification's title and artwork in step with the queue.
    _player.currentIndexStream.listen((index) {
      final items = queue.value;
      if (index != null && index >= 0 && index < items.length) {
        mediaItem.add(items[index]);
      }
    });

    _player.durationStream.listen((duration) {
      final current = mediaItem.value;
      // The server only knows a duration when ffprobe ran; the decoder always
      // does, so fill it in once playback starts or the scrubber has no scale.
      if (current != null && duration != null && current.duration != duration) {
        mediaItem.add(current.copyWith(duration: duration));
      }
    });
  }

  final AudioPlayer _player = AudioPlayer();

  /// Exposed so the UI can watch position without a second source of truth.
  AudioPlayer get player => _player;

  bool _sessionConfigured = false;

  /// Configures the platform audio session: ducking, transient loss from a
  /// navigation prompt, and pause-on-unplug all come from this.
  Future<void> _ensureSession() async {
    if (_sessionConfigured) return;
    final session = await AudioSession.instance;
    await session.configure(const AudioSessionConfiguration.music());
    _sessionConfigured = true;
  }

  /// Replaces the queue and starts at [initialIndex].
  Future<void> setQueueAndPlay(List<MediaItem> items, {int initialIndex = 0}) async {
    await _ensureSession();

    queue.add(items);
    if (items.isEmpty) {
      await _player.stop();
      return;
    }

    final safeIndex = initialIndex.clamp(0, items.length - 1);
    mediaItem.add(items[safeIndex]);

    await _player.setAudioSource(
      ConcatenatingAudioSource(
        children: items.map(_toSource).toList(growable: false),
      ),
      initialIndex: safeIndex,
      initialPosition: Duration.zero,
    );
    await _player.play();
  }

  /// Appends a track so it plays after the current one.
  Future<void> playNext(MediaItem item) async {
    final source = _player.audioSource;
    final items = List<MediaItem>.of(queue.value);
    final at = (_player.currentIndex ?? -1) + 1;

    items.insert(at.clamp(0, items.length), item);
    queue.add(items);

    if (source is ConcatenatingAudioSource) {
      await source.insert(at.clamp(0, source.length), _toSource(item));
    } else {
      await setQueueAndPlay(items, initialIndex: at);
    }
  }

  AudioSource _toSource(MediaItem item) => AudioSource.uri(Uri.parse(item.id), tag: item);

  @override
  Future<void> play() async {
    await _ensureSession();
    await _player.play();
  }

  @override
  Future<void> pause() => _player.pause();

  @override
  Future<void> stop() async {
    await _player.stop();
    playbackState.add(playbackState.value.copyWith(
      processingState: AudioProcessingState.idle,
      playing: false,
    ));
    await super.stop();
  }

  @override
  Future<void> seek(Duration position) => _player.seek(position);

  @override
  Future<void> skipToNext() => _player.seekToNext();

  @override
  Future<void> skipToPrevious() async {
    // What a "previous" button should do: restart the track if you are more
    // than a few seconds in, otherwise go back one.
    if (_player.position > const Duration(seconds: 3)) {
      await _player.seek(Duration.zero);
      return;
    }
    await _player.seekToPrevious();
  }

  @override
  Future<void> skipToQueueItem(int index) async {
    if (index < 0 || index >= queue.value.length) return;
    await _player.seek(Duration.zero, index: index);
    await _player.play();
  }

  @override
  Future<void> setShuffleMode(AudioServiceShuffleMode shuffleMode) async {
    final enabled = shuffleMode == AudioServiceShuffleMode.all;
    if (enabled) await _player.shuffle();
    await _player.setShuffleModeEnabled(enabled);
    playbackState.add(playbackState.value.copyWith(shuffleMode: shuffleMode));
  }

  @override
  Future<void> setRepeatMode(AudioServiceRepeatMode repeatMode) async {
    await _player.setLoopMode(switch (repeatMode) {
      AudioServiceRepeatMode.one => LoopMode.one,
      AudioServiceRepeatMode.all || AudioServiceRepeatMode.group => LoopMode.all,
      AudioServiceRepeatMode.none => LoopMode.off,
    });
    playbackState.add(playbackState.value.copyWith(repeatMode: repeatMode));
  }

  void _broadcastState(PlaybackEvent event) {
    final playing = _player.playing;
    playbackState.add(playbackState.value.copyWith(
      controls: [
        MediaControl.skipToPrevious,
        if (playing) MediaControl.pause else MediaControl.play,
        MediaControl.skipToNext,
        MediaControl.stop,
      ],
      systemActions: const {
        MediaAction.seek,
        MediaAction.seekForward,
        MediaAction.seekBackward,
      },
      // Which controls survive being collapsed into the compact notification.
      androidCompactActionIndices: const [0, 1, 2],
      processingState: switch (_player.processingState) {
        ProcessingState.idle => AudioProcessingState.idle,
        ProcessingState.loading => AudioProcessingState.loading,
        ProcessingState.buffering => AudioProcessingState.buffering,
        ProcessingState.ready => AudioProcessingState.ready,
        ProcessingState.completed => AudioProcessingState.completed,
      },
      playing: playing,
      updatePosition: _player.position,
      bufferedPosition: _player.bufferedPosition,
      speed: _player.speed,
      queueIndex: event.currentIndex,
    ));
  }
}

/// Starts the background audio service. Called once, at startup.
Future<AtticAudioHandler> initAudioService() async {
  return AudioService.init(
    builder: AtticAudioHandler.new,
    config: const AudioServiceConfig(
      androidNotificationChannelId: 'com.perleybrook.attic.playback',
      androidNotificationChannelName: 'Playback',
      androidNotificationOngoing: true,
      androidStopForegroundOnPause: true,
    ),
  );
}
