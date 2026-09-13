import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../features/auth/auth_controller.dart';
import '../../features/auth/connect_screen.dart';
import '../../features/auth/login_screen.dart';
import '../../features/music/player/player_screen.dart';
import '../../features/music/ui/album_detail_screen.dart';
import '../../features/music/ui/artist_albums_screen.dart';
import '../../features/music/ui/artists_screen.dart';
import '../../features/music/ui/playlist_detail_screen.dart';
import '../../features/music/ui/playlists_screen.dart';
import '../../features/photos/photos_screen.dart';
import '../../features/settings/settings_screen.dart';
import '../../features/video/video_screen.dart';
import '../flags.dart';
import '../layout/app_shell.dart';
import '../layout/destinations.dart';

/// The app's routes.
///
/// The four top-level destinations live inside a [ShellRoute] so the navigation
/// bar (and the mini-player docked above it) stay mounted while switching
/// between them. Onboarding sits outside the shell, and a redirect keeps the
/// two apart: there is no state in which a signed-out user can reach the
/// library, or a signed-in one is stuck on the login screen.
final routerProvider = Provider<GoRouter>((ref) => createRouter(ref));

GoRouter createRouter(Ref ref) {
  return GoRouter(
    initialLocation: '/music',
    refreshListenable: _AuthListenable(ref),
    redirect: (context, state) {
      if (!Flags.auth) return null;

      final stage = ref.read(authControllerProvider).stage;
      final location = state.matchedLocation;

      return switch (stage) {
        AuthStage.restoring => location == '/splash' ? null : '/splash',
        AuthStage.needsServer => location == '/connect' ? null : '/connect',
        AuthStage.needsLogin => location == '/login' ? null : '/login',
        AuthStage.authenticated =>
          const {'/splash', '/connect', '/login'}.contains(location) ? '/music' : null,
      };
    },
    routes: <RouteBase>[
      GoRoute(path: '/splash', builder: (context, state) => const _SplashScreen()),
      GoRoute(path: '/connect', builder: (context, state) => const ConnectScreen()),
      GoRoute(path: '/login', builder: (context, state) => const LoginScreen()),

      // Full-screen player: outside the shell, so it covers the mini-player
      // rather than sitting above it.
      GoRoute(path: '/music/player', builder: (context, state) => const PlayerScreen()),

      ShellRoute(
        builder: (context, state, child) => _Shell(state: state, child: child),
        routes: <RouteBase>[
          GoRoute(
            path: '/photos',
            pageBuilder: (context, state) =>
                const NoTransitionPage<void>(child: PhotosScreen()),
          ),
          GoRoute(
            path: '/music',
            pageBuilder: (context, state) =>
                const NoTransitionPage<void>(child: ArtistsScreen()),
            routes: <RouteBase>[
              GoRoute(
                path: 'artists/:id',
                builder: (context, state) =>
                    ArtistAlbumsScreen(artistId: state.pathParameters['id']!),
              ),
              GoRoute(
                path: 'albums/:id',
                builder: (context, state) =>
                    AlbumDetailScreen(albumId: state.pathParameters['id']!),
              ),
              GoRoute(
                path: 'playlists',
                builder: (context, state) => const PlaylistsScreen(),
                routes: <RouteBase>[
                  GoRoute(
                    path: ':id',
                    builder: (context, state) =>
                        PlaylistDetailScreen(playlistId: state.pathParameters['id']!),
                  ),
                ],
              ),
            ],
          ),
          GoRoute(
            path: '/video',
            pageBuilder: (context, state) =>
                const NoTransitionPage<void>(child: VideoScreen()),
          ),
          GoRoute(
            path: '/settings',
            pageBuilder: (context, state) =>
                const NoTransitionPage<void>(child: SettingsScreen()),
          ),
        ],
      ),
    ],
  );
}

/// Bridges Riverpod's auth state to go_router's refresh mechanism, so a
/// session expiring anywhere in the app immediately re-evaluates the redirect.
class _AuthListenable extends ChangeNotifier {
  _AuthListenable(Ref ref) {
    ref.listen<AuthState>(
      authControllerProvider,
      (previous, next) {
        if (previous?.stage != next.stage) notifyListeners();
      },
    );
  }
}

class _Shell extends StatelessWidget {
  const _Shell({required this.state, required this.child});

  final GoRouterState state;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return AppShell(
      selectedIndex: destinationIndexFor(state.uri.path),
      onDestinationSelected: (index) => context.go(kDestinations[index].route),
      child: child,
    );
  }
}

class _SplashScreen extends StatelessWidget {
  const _SplashScreen();

  @override
  Widget build(BuildContext context) {
    return const Scaffold(body: Center(child: CircularProgressIndicator()));
  }
}

/// Map a location to the index of the destination that owns it.
/// Unknown locations fall back to the first destination.
int destinationIndexFor(String location) {
  final index = kDestinations.indexWhere((d) => location.startsWith(d.route));
  return index < 0 ? 0 : index;
}
