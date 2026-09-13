import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// User-visible app settings that survive restarts.
///
/// Theme mode follows the system by default and is overridable in Settings,
/// as the design system requires.
class Settings {
  const Settings({this.themeMode = ThemeMode.system});

  final ThemeMode themeMode;

  Settings copyWith({ThemeMode? themeMode}) =>
      Settings(themeMode: themeMode ?? this.themeMode);
}

class SettingsController extends StateNotifier<Settings> {
  SettingsController(this._prefs) : super(const Settings()) {
    _load();
  }

  static const _themeModeKey = 'theme_mode';

  final Future<SharedPreferences> _prefs;

  Future<void> _load() async {
    final prefs = await _prefs;
    final stored = prefs.getString(_themeModeKey);
    if (stored == null) return;
    state = state.copyWith(
      themeMode: ThemeMode.values.firstWhere(
        (m) => m.name == stored,
        orElse: () => ThemeMode.system,
      ),
    );
  }

  Future<void> setThemeMode(ThemeMode mode) async {
    state = state.copyWith(themeMode: mode);
    final prefs = await _prefs;
    await prefs.setString(_themeModeKey, mode.name);
  }
}

final sharedPreferencesProvider = Provider<Future<SharedPreferences>>(
  (ref) => SharedPreferences.getInstance(),
);

final settingsProvider =
    StateNotifierProvider<SettingsController, Settings>((ref) {
  return SettingsController(ref.watch(sharedPreferencesProvider));
});
