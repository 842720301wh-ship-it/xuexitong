//go:build android && cgo

#include <jni.h>
#include "_cgo_export.h"

JNIEXPORT jint JNICALL Java_com_tiantianxiangshang_app_NativeBackend_startBackend(
    JNIEnv *env, jclass type, jstring directory, jstring token) {
    const char *value = (*env)->GetStringUTFChars(env, directory, NULL);
    if (value == NULL) return 0;
    const char *secret = (*env)->GetStringUTFChars(env, token, NULL);
    if (secret == NULL) { (*env)->ReleaseStringUTFChars(env, directory, value); return 0; }
    int port = StartTiantian((char *)value, (char *)secret);
    (*env)->ReleaseStringUTFChars(env, token, secret);
    (*env)->ReleaseStringUTFChars(env, directory, value);
    return port;
}
