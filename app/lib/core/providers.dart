import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../features/music/data/music_repository.dart';
import 'api/api_client.dart';
import 'auth/auth_repository.dart';
import 'auth/token_store.dart';

/// The composition root. Every dependency is overridable, which is how the
/// widget tests swap in a canned server.

final sessionStoreProvider = Provider<SessionStore>((ref) => TokenStore());

final apiClientProvider = Provider<ApiClient>((ref) => ApiClient());

final authRepositoryProvider = Provider<AuthRepository>((ref) {
  return AuthRepository(
    client: ref.watch(apiClientProvider),
    store: ref.watch(sessionStoreProvider),
  );
});

final musicRepositoryProvider = Provider<MusicRepository>((ref) {
  return MusicRepository(ref.watch(apiClientProvider));
});
