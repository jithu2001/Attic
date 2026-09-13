import 'package:audio_service/audio_service.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/auth/auth_repository.dart';
import '../../../core/permissions/notification_permission.dart';
import '../../../core/providers.dart';
import '../data/models.dart';
import 'audio_handler.dart';

/// Set once at startup by [initAudioService]. The provider throws if something
/// reaches for the player before the service is up, which is a wiring bug
/// rather than a runtime condition to handle.
final audioHandlerProvider = Provider<AudioHandler>((ref) {
  throw UnimplementedError('audioHandlerProvider must be overridden at startup');
});

/// Turns library tracks into a playing queue.
///
/// The mapping to [MediaItem] happens here, in one place, because the media
/// item is what the lock screen renders: get it wrong and the notification
/// shows the wrong song while the right one plays.
class PlayerController {
  PlayerController({
    required AudioHandler handler,
    required AuthRepository auth,
    required NotificationPermissionController notifications,
  })  : _handler = handler,
        _auth = auth,
        _notifications = notifications;

  final AudioHandler _handler;
  final AuthRepository _auth;
  final NotificationPermissionController _notifications;

  /// Plays [tracks] starting at [index].
  Future<void> playQueue(List<Track> tracks, {int index = 0}) async {
    if (tracks.isEmpty) return;

    // Ask for notification permission here, at the first moment it means
    // anything: on Android 13+ the media notification is where the
    // lock-screen and headset controls live. A refusal only costs those
    // controls, so it must never stop playback.
    await _notifications.ensure();

    // Playback URLs carry a media token, so refresh it before building a queue
    // rather than discovering it expired three tracks in.
    await _auth.ensureMediaToken();

    final items = tracks.map(_toMediaItem).toList(growable: false);
    final handler = _handler;
    if (handler is AtticAudioHandler) {
      await handler.setQueueAndPlay(items, initialIndex: index);
      return;
    }
    await handler.updateQueue(items);
    await handler.skipToQueueItem(index);
  }

  /// Queues one track to play after the current one.
  Future<void> playNext(Track track) async {
    await _auth.ensureMediaToken();
    final item = _toMediaItem(track);
    final handler = _handler;
    if (handler is AtticAudioHandler) {
      await handler.playNext(item);
      return;
    }
    await handler.addQueueItem(item);
  }

  Future<void> play() => _handler.play();
  Future<void> pause() => _handler.pause();
  Future<void> seek(Duration position) => _handler.seek(position);
  Future<void> skipToNext() => _handler.skipToNext();
  Future<void> skipToPrevious() => _handler.skipToPrevious();
  Future<void> skipToQueueItem(int index) => _handler.skipToQueueItem(index);

  Future<void> toggleShuffle(bool enabled) => _handler.setShuffleMode(
        enabled ? AudioServiceShuffleMode.all : AudioServiceShuffleMode.none,
      );

  Future<void> cycleRepeat(AudioServiceRepeatMode current) =>
      _handler.setRepeatMode(switch (current) {
        AudioServiceRepeatMode.none => AudioServiceRepeatMode.all,
        AudioServiceRepeatMode.all => AudioServiceRepeatMode.one,
        _ => AudioServiceRepeatMode.none,
      });

  MediaItem _toMediaItem(Track track) => MediaItem(
        // The id is the URL the player fetches, signed with a media token
        // because a platform media stack cannot set an Authorization header.
        id: _auth.signMediaUrl(track.audioUrl),
        title: track.title,
        artist: track.displayArtist,
        album: track.album,
        duration: track.duration,
        artUri: track.coverUrl == null
            ? null
            : Uri.parse(_auth.signMediaUrl(track.coverUrl!)),
        extras: <String, dynamic>{'track_id': track.id, 'album_id': track.albumId},
      );
}

final playerControllerProvider = Provider<PlayerController>((ref) {
  return PlayerController(
    handler: ref.watch(audioHandlerProvider),
    auth: ref.watch(authRepositoryProvider),
    notifications: ref.watch(notificationPermissionProvider.notifier),
  );
});

/// What is playing right now, or null when nothing is.
final currentMediaItemProvider = StreamProvider<MediaItem?>((ref) {
  return ref.watch(audioHandlerProvider).mediaItem;
});

/// Transport state: playing, buffering, shuffle and repeat modes.
final playbackStateProvider = StreamProvider<PlaybackState>((ref) {
  return ref.watch(audioHandlerProvider).playbackState;
});

/// The queue as the player sees it.
final playbackQueueProvider = StreamProvider<List<MediaItem>>((ref) {
  return ref.watch(audioHandlerProvider).queue;
});
