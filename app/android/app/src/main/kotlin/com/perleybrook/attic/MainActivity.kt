package com.perleybrook.attic

import com.ryanheise.audioservice.AudioServiceActivity

// AudioServiceActivity rather than FlutterActivity: audio_service needs the
// activity to hand playback off to its background service, which is what keeps
// music going once the app is backgrounded or the screen is locked.
class MainActivity : AudioServiceActivity()
