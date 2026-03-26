# Add project specific ProGuard rules here.
# You can control the set of applied configuration files using the
# proguardFiles setting in build.gradle.

# Keep model classes
-keep class com.chatapp.domain.model.** { *; }
-keep class com.chatapp.data.remote.dto.** { *; }
-keep class com.chatapp.data.local.entity.** { *; }

# Keep Retrofit interfaces
-keep interface com.chatapp.data.remote.api.ChatApi { *; }

# Keep WebSocket related classes
-keep class okhttp3.** { *; }
-keep class okhttp3.internal.** { *; }
-keep interface okhttp3.** { *; }
-dontwarn okhttp3.**

# Keep Okio classes
-keep class okio.** { *; }
-dontwarn okio.**

# Keep Gson classes
-keep class com.google.gson.** { *; }
-keep class * implements com.google.gson.TypeAdapterFactory
-keep class * implements com.google.gson.JsonSerializer
-keep class * implements com.google.gson.JsonDeserializer

# Keep Hilt generated classes
-keep class dagger.hilt.** { *; }
-keep class * extends dagger.hilt.internal.GeneratedComponent { *; }

# Keep Room entities
-keepclassmembers class * {
    @androidx.room.PrimaryKey <fields>;
    @androidx.room.ColumnInfo <fields>;
    @androidx.room.Embedded <fields>;
}

# Keep WebSocket message classes
-keep class com.chatapp.websocket.model.** { *; }
-keep class com.chatapp.websocket.message.** { *; }

# Keep encrypted message classes
-keep class com.chatapp.encryption.** { *; }

# Keep coroutines
-keepnames class kotlinx.coroutines.** { *; }
-keepclassmembernames class kotlinx.coroutines.** { *; }

# Keep RxJava
-keep class rx.** { *; }
-dontwarn rx.**

# Keep lifecycle components
-keep class * extends androidx.lifecycle.ViewModel
-keep class * extends androidx.lifecycle.AndroidViewModel
-keep class * extends androidx.lifecycle.LiveData
-keep class * extends androidx.lifecycle.MutableLiveData

# Keep navigation components
-keep class * extends androidx.navigation.NavController
-keep class * extends androidx.navigation.NavGraph

# Keep data binding classes
-keep class androidx.databinding.** { *; }
-keep class * extends androidx.databinding.ViewDataBinding
-keep class * extends androidx.databinding.Bindable

# Keep custom annotations
-keep @interface androidx.annotation.Keep
-keep @interface androidx.annotation.Nullable
-keep @interface androidx.annotation.NonNull

# Keep enum classes
-keepclassmembers enum * {
    public static **[] values();
    public static ** valueOf(java.lang.String);
}

# Keep Parcelable implementations
-keep class * implements android.os.Parcelable {
    public static final ** CREATOR;
}

# Keep Serializable implementations
-keepclassmembers class * implements java.io.Serializable {
    static final long serialVersionUID;
    private static final java.io.ObjectStreamField[] serialPersistentFields;
    private void writeObject(java.io.ObjectOutputStream);
    private void readObject(java.io.ObjectInputStream);
    java.lang.Object writeReplace();
    java.lang.Object readResolve();
}

# Keep enums
-keepclassmembers enum * {
    public static **[] values();
    public static ** valueOf(java.lang.String);
}

# Keep R8 annotations
-keepattributes *Annotation*
-keepattributes Signature
-keepattributes Exceptions
-keepattributes InnerClasses
-keepattributes EnclosingMethod
-keepattributes SourceFile
-keepattributes LineNumberTable

# Remove logging calls
-assumenosideeffects class android.util.Log {
    public static boolean isLoggable(java.lang.String, int);
    public static int v(...);
    public static int i(...);
    public static int w(...);
    public static int d(...);
    public static int e(...);
}

# Retrofit
-dontwarn retrofit2.**
-keep class retrofit2.** { *; }
-keepattributes Signature
-keepattributes Exceptions

# Gson
-keep class sun.misc.Unsafe { *; }
-keep class com.google.gson.** { *; }

# OkHttp
-dontwarn okhttp3.**
-dontwarn okio.**

# Scarlet WebSocket
-dontwarn com.tinder.scarlet.**
-keep class com.tinder.scarlet.** { *; }

# Room
-dontwarn androidx.room.paging.**

# Kotlin Coroutines
-keepnames class kotlinx.coroutines.internal.MainDispatcherFactory {}
-keepnames class kotlinx.coroutines.internal.AndroidExceptionPreHandler {}

# Firebase
-keep class com.google.firebase.** { *; }
-dontwarn com.google.firebase.**

# Hilt
-keep class dagger.hilt.** { *; }
-keep class * extends dagger.hilt.internal.GeneratedComponent { *; }
-keepclassmembers,allowobfuscation class * {
    @javax.inject.* *;
    @dagger.* *;
    <init>();
}

# Keep native methods
-keepclasseswithmembernames class * {
    native <methods>;
}
