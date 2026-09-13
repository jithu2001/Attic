import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/api_client.dart';

/// Where the user is in the connect-then-sign-in flow.
enum AuthStage {
  /// No server address recorded yet.
  needsServer,

  /// Server known, no valid session.
  needsLogin,

  /// Signed in.
  authenticated,
}

class AuthState {
  const AuthState({
    this.stage = AuthStage.needsServer,
    this.serverUrl,
    this.username,
    this.busy = false,
    this.error,
  });

  final AuthStage stage;
  final String? serverUrl;
  final String? username;
  final bool busy;
  final String? error;

  AuthState copyWith({
    AuthStage? stage,
    String? serverUrl,
    String? username,
    bool? busy,
    String? error,
    bool clearError = false,
  }) {
    return AuthState(
      stage: stage ?? this.stage,
      serverUrl: serverUrl ?? this.serverUrl,
      username: username ?? this.username,
      busy: busy ?? this.busy,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

/// Drives the connect + sign-in flow.
///
/// This is the skeleton: [connect] really does reach the server's `/ping` to
/// validate the address, but [signIn] is a stub until the auth phase ships
/// tokens. The whole flow is gated behind `Flags.auth`, so nothing here is
/// reachable in a default build.
class AuthController extends StateNotifier<AuthState> {
  AuthController(this._api) : super(const AuthState());

  final ApiClient _api;

  /// Verify that [address] speaks Attic, and remember it.
  Future<bool> connect(String address) async {
    if (address.trim().isEmpty) {
      state = state.copyWith(error: 'Enter your server address.');
      return false;
    }

    state = state.copyWith(busy: true, clearError: true);
    _api.baseUrl = address;
    try {
      await _api.getJson('/ping');
      state = state.copyWith(
        busy: false,
        stage: AuthStage.needsLogin,
        serverUrl: _api.baseUrl,
      );
      return true;
    } on ApiError catch (e) {
      state = state.copyWith(busy: false, error: e.message);
      return false;
    } catch (_) {
      state = state.copyWith(
        busy: false,
        error: 'Could not reach that server. Check the address and that you '
            'are on the tailnet.',
      );
      return false;
    }
  }

  /// Stub: the auth phase replaces this with a real token exchange, refresh
  /// token storage in flutter_secure_storage, and a dio auth interceptor.
  Future<bool> signIn({required String username, required String password}) async {
    state = state.copyWith(
      busy: false,
      error: 'Sign-in arrives with the authentication phase.',
    );
    return false;
  }
}

final apiClientProvider = Provider<ApiClient>((ref) => ApiClient());

final authControllerProvider =
    StateNotifierProvider<AuthController, AuthState>((ref) {
  return AuthController(ref.watch(apiClientProvider));
});
