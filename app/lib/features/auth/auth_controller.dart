import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/api_error.dart';
import '../../core/auth/auth_repository.dart';
import '../../core/providers.dart';
import 'device_name.dart';

/// Where the user is in the connect-then-sign-in flow.
enum AuthStage {
  /// Still restoring a saved session.
  restoring,

  /// No server address recorded yet.
  needsServer,

  /// Server known, no valid session.
  needsLogin,

  /// Signed in.
  authenticated,
}

@immutable
class AuthState {
  const AuthState({
    this.stage = AuthStage.restoring,
    this.serverUrl,
    this.account,
    this.busy = false,
  });

  final AuthStage stage;
  final String? serverUrl;
  final Account? account;

  /// True while a connect or sign-in request is in flight, so the button can
  /// show a spinner and refuse a second tap.
  final bool busy;

  AuthState copyWith({
    AuthStage? stage,
    String? serverUrl,
    Account? account,
    bool? busy,
    bool clearAccount = false,
  }) {
    return AuthState(
      stage: stage ?? this.stage,
      serverUrl: serverUrl ?? this.serverUrl,
      account: clearAccount ? null : (account ?? this.account),
      busy: busy ?? this.busy,
    );
  }
}

/// Drives connect, sign-in and sign-out.
///
/// Errors are thrown rather than parked in the state: the screens show them as
/// SnackBars, and a stale error message surviving in state is worse than none.
class AuthController extends StateNotifier<AuthState> {
  AuthController(this._auth) : super(const AuthState()) {
    _auth.onSessionLost = _handleSessionLost;
  }

  final AuthRepository _auth;

  /// Restores a saved session, if there is one. Called once at startup.
  Future<void> restore() async {
    final session = await _auth.restore();
    if (session == null) {
      state = state.copyWith(
        stage: _auth.serverUrl == null ? AuthStage.needsServer : AuthStage.needsLogin,
        serverUrl: _auth.serverUrl,
      );
      return;
    }

    // The stored tokens might be stale; /me settles it, refreshing on the way
    // if the access token has simply expired.
    try {
      final account = await _auth.me();
      state = state.copyWith(
        stage: AuthStage.authenticated,
        serverUrl: session.serverUrl,
        account: account,
      );
    } on ApiError catch (e) {
      // A network failure must not throw away a good session: the user may
      // just be off the tailnet. Only an actual rejection signs them out.
      if (e.isUnauthorized) {
        await _auth.signOut();
        state = state.copyWith(
          stage: AuthStage.needsLogin,
          serverUrl: session.serverUrl,
          clearAccount: true,
        );
      } else {
        state = state.copyWith(
          stage: AuthStage.authenticated,
          serverUrl: session.serverUrl,
        );
      }
    }
  }

  /// Verifies a server address and moves on to sign-in.
  Future<void> connect(String address) async {
    state = state.copyWith(busy: true);
    try {
      await _auth.connect(address);
      state = state.copyWith(
        stage: AuthStage.needsLogin,
        serverUrl: _auth.serverUrl,
        busy: false,
      );
    } catch (_) {
      state = state.copyWith(busy: false);
      rethrow;
    }
  }

  /// Signs in and fetches the media token playback needs.
  Future<void> signIn({required String username, required String password}) async {
    state = state.copyWith(busy: true);
    try {
      final account = await _auth.signIn(
        username: username,
        password: password,
        deviceName: await describeDevice(),
      );
      await _auth.ensureMediaToken(force: true);
      state = state.copyWith(
        stage: AuthStage.authenticated,
        account: account,
        busy: false,
      );
    } catch (_) {
      state = state.copyWith(busy: false);
      rethrow;
    }
  }

  /// Revokes this device and returns to the login screen.
  Future<void> signOut() async {
    await _auth.signOut();
    state = state.copyWith(
      stage: AuthStage.needsLogin,
      clearAccount: true,
    );
  }

  /// Forgets the server too, so a different one can be entered.
  void changeServer() {
    state = state.copyWith(stage: AuthStage.needsServer);
  }

  void _handleSessionLost() {
    if (!mounted) return;
    state = state.copyWith(stage: AuthStage.needsLogin, clearAccount: true);
  }
}

final authControllerProvider =
    StateNotifierProvider<AuthController, AuthState>((ref) {
  return AuthController(ref.watch(authRepositoryProvider));
});
