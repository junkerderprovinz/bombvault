plugins {
    id("com.android.application") version "9.4.1"
}

// The app and the server it ships with carry one number, set in
// gradle.properties. A test build passes its own, below the first release.
val release = findProperty("bombvaultVersion") as String
val (major, minor, patch) = release.split(".").map { it.toInt() }

android {
    namespace = "bombvault.halleluja.design"
    compileSdk = 37

    defaultConfig {
        applicationId = "bombvault.halleluja.design"
        // From 29 a certificate can be read back from an SSL error, which is
        // what pinning a self-signed server needs.
        minSdk = 29
        targetSdk = 36
        versionCode = major * 10000 + minor * 100 + patch
        versionName = release
    }

    // A committed debug key keeps test builds updatable across machines. It is
    // Android's published debug key and grants nothing.
    signingConfigs {
        getByName("debug") {
            storeFile = rootProject.file("debug.keystore")
            storePassword = "android"
            keyAlias = "androiddebugkey"
            keyPassword = "android"
        }
        create("release") {
            val store = System.getenv("ANDROID_KEYSTORE")
            if (store != null) {
                storeFile = file(store)
                storePassword = System.getenv("ANDROID_KEYSTORE_PASSWORD")
                keyAlias = System.getenv("ANDROID_KEY_ALIAS")
                keyPassword = System.getenv("ANDROID_KEY_PASSWORD")
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"))
            // Without the key a release build is a local test build, and an
            // unsigned APK does not install. The release workflow refuses a
            // debug-signed result.
            signingConfig = signingConfigs.getByName(
                if (System.getenv("ANDROID_KEYSTORE") != null) "release" else "debug",
            )
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlin {
        compilerOptions {
            jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17)
        }
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.19.1")
    implementation("androidx.activity:activity-ktx:1.13.0")
    implementation("androidx.webkit:webkit:1.14.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.11.0")
    // The relay is a WebSocket, which the platform has no client for.
    implementation("com.squareup.okhttp3:okhttp:5.5.0")

    testImplementation("junit:junit:4.13.2")
}
