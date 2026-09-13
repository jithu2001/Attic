import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../features/auth/connect_screen.dart';
import '../../features/auth/login_screen.dart';
import '../../features/music/music_screen.dart';
import '../../features/photos/photos_screen.dart';
import '../../features/settings/settings_screen.dart';
import '../../features/video/video_screen.dart';
import '../flags.dart';
import '../layout/app_shell.dart';
import '../layout/destinations.dart';

/// The app's routes.
///
/// The four top-level destinations live inside a [ShellRoute] so the
/// navigation bar / rail stays mounted while switching between them. The
/// onboarding routes sit outside the shell and only exist when `Flags.auth`
/// is on.
GoRouter createRouter() {
  return GoRouter(
    initialLocation: Flags.auth ? '/connect' : '/photos',
    routes: <RouteBase>[
      if (Flags.auth) ...<RouteBase>[
        GoRoute(
          path: '/connect',
          builder: (context, state) => const ConnectScreen(),
        ),
        GoRoute(
          path: '/login',
          builder: (context, state) => const LoginScreen(),
        ),
      ],
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
                const NoTransitionPage<void>(child: MusicScreen()),
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

class _Shell extends StatelessWidget {
  const _Shell({required this.state, required this.child});

  final GoRouterState state;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return AppShell(
      selectedIndex: destinationIndexFor(state.uri.path),
      onDestinationSelected: (index) =>
          context.go(kDestinations[index].route),
      child: child,
    );
  }
}

/// Map a location to the index of the destination that owns it.
/// Unknown locations fall back to the first destination.
int destinationIndexFor(String location) {
  final index =
      kDestinations.indexWhere((d) => location.startsWith(d.route));
  return index < 0 ? 0 : index;
}
