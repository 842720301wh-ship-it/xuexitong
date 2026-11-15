package com.tiantianxiangshang.app;

import android.content.Context;
import java.security.SecureRandom;
import android.util.Base64;
import org.json.JSONObject;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.io.ByteArrayOutputStream;
import java.io.InputStream;

final class NativeBackend {
    private static native int startBackend(String directory, String token);
    private static int port;
    private static String token;
    static synchronized String ensure(Context context) {
        if (port == 0) {
            byte[] bytes = new byte[32]; new SecureRandom().nextBytes(bytes);
            token = Base64.encodeToString(bytes, Base64.NO_WRAP);
            System.loadLibrary("tiantian");
            port = startBackend(context.getFilesDir().getAbsolutePath(), token);
        }
        if (port <= 0) throw new IllegalStateException("本机服务启动失败");
        return "http://127.0.0.1:" + port;
    }
    static JSONObject post(Context context, String path, JSONObject body) throws Exception {
        HttpURLConnection connection = (HttpURLConnection) new URL(ensure(context) + path).openConnection();
        connection.setConnectTimeout(10000); connection.setReadTimeout(15000);
        connection.setInstanceFollowRedirects(false); connection.setRequestMethod("POST");
        connection.setRequestProperty("X-Tiantian-Native", token);
        connection.setRequestProperty("Content-Type", "application/json"); connection.setDoOutput(true);
        try {
            try (java.io.OutputStream out = connection.getOutputStream()) { out.write(body.toString().getBytes(StandardCharsets.UTF_8)); }
            int code = connection.getResponseCode();
            InputStream input = code == 200 ? connection.getInputStream() : connection.getErrorStream();
            ByteArrayOutputStream bytes = new ByteArrayOutputStream();
            if (input != null) try (InputStream in = input) {
                byte[] chunk = new byte[8192]; int count;
                while ((count = in.read(chunk)) != -1) { bytes.write(chunk, 0, count); if (bytes.size() > 1048576) throw new java.io.IOException("响应过大"); }
            }
            JSONObject result = new JSONObject(bytes.toString("UTF-8"));
            if (code != 200) throw new java.io.IOException(result.optString("error", "本机执行器请求失败"));
            return result;
        } finally { connection.disconnect(); }
    }
}
