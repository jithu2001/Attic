/// Compile-time feature flags.
///
/// Unfinished UI ships behind a flag that is **off by default**, so `main` is
/// always demoable. A flag flips to on in the phase that finishes its feature.
/// Flip one locally while working on it, or at build time:
///
/// ```sh
/// flutter run --dart-define=ATTIC_AUTH=true
/// ```
///
/// Each flag is a `static const bool`, which lets the Dart compiler tree-shake
/// the disabled branches out of release builds entirely.
class Flags {
  const Flags._();

  /// Server-address entry, sign-in and session refresh. Shipped.
  static const bool auth =
      bool.fromEnvironment('ATTIC_AUTH', defaultValue: true);

  /// Photo library: timeline, albums, viewer, backup.
  static const bool photos =
      bool.fromEnvironment('ATTIC_PHOTOS', defaultValue: false);

  /// Music library, search, playlists and background playback. Shipped.
  static const bool music =
      bool.fromEnvironment('ATTIC_MUSIC', defaultValue: true);

  /// Video library and player.
  static const bool video =
      bool.fromEnvironment('ATTIC_VIDEO', defaultValue: false);

  /// Background camera-roll backup (Android workmanager / iOS BGTask).
  static const bool backgroundSync =
      bool.fromEnvironment('ATTIC_BACKGROUND_SYNC', defaultValue: false);
}
