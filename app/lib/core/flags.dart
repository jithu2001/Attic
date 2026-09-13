/// Compile-time feature flags.
///
/// Unfinished UI ships behind a flag that is **off by default**, so `main` is
/// always demoable. Flip a flag locally while working on it, or at build time:
///
/// ```sh
/// flutter run --dart-define=ATTIC_AUTH=true
/// ```
///
/// Each flag is a `static const bool`, which lets the Dart compiler tree-shake
/// the disabled branches out of release builds entirely.
class Flags {
  const Flags._();

  /// Server-address entry and login. Stubbed in the skeleton phase.
  static const bool auth =
      bool.fromEnvironment('ATTIC_AUTH', defaultValue: false);

  /// Photo library: timeline, albums, viewer, backup.
  static const bool photos =
      bool.fromEnvironment('ATTIC_PHOTOS', defaultValue: false);

  /// Music library and player.
  static const bool music =
      bool.fromEnvironment('ATTIC_MUSIC', defaultValue: false);

  /// Video library and player.
  static const bool video =
      bool.fromEnvironment('ATTIC_VIDEO', defaultValue: false);

  /// Background camera-roll backup (Android workmanager / iOS BGTask).
  static const bool backgroundSync =
      bool.fromEnvironment('ATTIC_BACKGROUND_SYNC', defaultValue: false);
}
