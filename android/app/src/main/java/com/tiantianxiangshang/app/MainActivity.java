package com.tiantianxiangshang.app;

import android.app.Activity;
import android.content.Intent;
import android.graphics.Color;
import android.net.Uri;
import android.os.Bundle;
import android.view.Gravity;
import android.view.View;
import android.view.WindowInsets;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.FrameLayout;
import android.widget.TextView;
import android.widget.Toast;
import java.io.ByteArrayInputStream;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.util.Collections;

public final class MainActivity extends Activity {

    private static final int SAVE_IMAGE = 1;
    private WebView webView;
    private String origin;
    private String pendingImage;

    @Override public void onCreate(Bundle state) {
        super.onCreate(state);
        if (state != null) pendingImage = state.getString("pendingImage");
        getWindow().setStatusBarColor(Color.WHITE);
        getWindow().setNavigationBarColor(Color.WHITE);
        getWindow().getDecorView().setSystemUiVisibility(View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR | View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR);
        FrameLayout frame = new FrameLayout(this);
        frame.setBackgroundColor(Color.WHITE);
        frame.setOnApplyWindowInsetsListener((view, insets) -> {
            if (android.os.Build.VERSION.SDK_INT >= 30) {
                android.graphics.Insets bars = insets.getInsets(WindowInsets.Type.systemBars() | WindowInsets.Type.ime());
                view.setPadding(bars.left, bars.top, bars.right, bars.bottom);
            } else {
                view.setPadding(insets.getSystemWindowInsetLeft(), insets.getSystemWindowInsetTop(), insets.getSystemWindowInsetRight(), insets.getSystemWindowInsetBottom());
            }
            return insets;
        });
        TextView loading = new TextView(this);
        loading.setText("正在启动天天向上…");
        loading.setGravity(Gravity.CENTER);
        frame.addView(loading, new FrameLayout.LayoutParams(-1, -1));
        setContentView(frame);
        new Thread(() -> {
            String result = null;
            try { result = NativeBackend.ensure(this); } catch (Throwable error) { android.util.Log.e("Tiantian", "Cannot load local service"); }
            final String address = result;
            runOnUiThread(() -> {
                if (isFinishing() || isDestroyed()) return;
                if (address == null) {
                    loading.setText("本机服务启动失败，请关闭应用后重试。");
                    return;
                }
                origin = address;
                webView = new WebView(this);
                WebView.setWebContentsDebuggingEnabled((getApplicationInfo().flags & android.content.pm.ApplicationInfo.FLAG_DEBUGGABLE) != 0);
                WebSettings settings = webView.getSettings();
                settings.setJavaScriptEnabled(true);
                settings.setAllowFileAccess(false);
                settings.setAllowContentAccess(false);
                settings.setMixedContentMode(WebSettings.MIXED_CONTENT_NEVER_ALLOW);
                settings.setSaveFormData(false);
                webView.addJavascriptInterface(new AndroidBridge(), "TiantianAndroid");
                webView.setWebViewClient(new WebViewClient() {
                    @Override public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
                        Uri uri = request.getUrl();
                        if (local(uri)) return false;
                        if (request.isForMainFrame() && request.hasGesture() && "https".equals(uri.getScheme())) {
                            try {
                                startActivity(new Intent(Intent.ACTION_VIEW, uri).addCategory(Intent.CATEGORY_BROWSABLE));
                            } catch (android.content.ActivityNotFoundException error) {
                                toast("未找到可打开链接的浏览器");
                            }
                        }
                        return true;
                    }
                    @Override public WebResourceResponse shouldInterceptRequest(WebView view, WebResourceRequest request) {
                        if (local(request.getUrl())) return null;
                        return new WebResourceResponse("text/plain", "UTF-8", 403, "Blocked", Collections.emptyMap(), new ByteArrayInputStream(new byte[0]));
                    }
                });
                webView.setDownloadListener((url, userAgent, disposition, mimeType, size) -> {
                    Uri uri = Uri.parse(url);
                    if (!local(uri) || !("/about/sponsor.jpg".equals(uri.getPath()) || "/about/contact.jpg".equals(uri.getPath()))) return;
                    if (pendingImage != null) return;
                    pendingImage = uri.getPath();
                    Intent save = new Intent(Intent.ACTION_CREATE_DOCUMENT).addCategory(Intent.CATEGORY_OPENABLE)
                        .setType("image/jpeg").putExtra(Intent.EXTRA_TITLE, "天天向上-" + uri.getLastPathSegment());
                    try { startActivityForResult(save, SAVE_IMAGE); }
                    catch (android.content.ActivityNotFoundException error) { pendingImage = null; toast("未找到文件保存程序"); }
                });
                frame.removeAllViews();
                frame.addView(webView, new FrameLayout.LayoutParams(-1, -1));
                webView.loadUrl(origin + "/#my");
            });
        }, "tiantian-start").start();
    }

    public final class AndroidBridge {
        @android.webkit.JavascriptInterface public String status() {
            try {
                android.os.PowerManager pm = (android.os.PowerManager) getSystemService(POWER_SERVICE);
                return new org.json.JSONObject().put("notifications", getSystemService(android.app.NotificationManager.class).areNotificationsEnabled())
                    .put("battery_exempt", pm.isIgnoringBatteryOptimizations(getPackageName())).put("running", !LearningService.activeTask.isEmpty()).toString();
            } catch (Exception e) { return "{}"; }
        }
        @android.webkit.JavascriptInterface public void startTask(String id) {
            if (id == null || !id.matches("[a-f0-9]{32}")) return;
            runOnUiThread(() -> {
                if (android.os.Build.VERSION.SDK_INT >= 33 && checkSelfPermission(android.Manifest.permission.POST_NOTIFICATIONS) != android.content.pm.PackageManager.PERMISSION_GRANTED)
                    requestPermissions(new String[]{android.Manifest.permission.POST_NOTIFICATIONS}, 11);
                try { startForegroundService(new Intent(MainActivity.this, LearningService.class).putExtra("task_id", id)); }
                catch (Exception e) { toast("前台服务启动失败，请检查手机后台运行权限"); }
            });
        }
        @android.webkit.JavascriptInterface public void stopTask(String id) {
            runOnUiThread(() -> { if (id.equals(LearningService.activeTask)) startService(new Intent(MainActivity.this, LearningService.class).setAction(LearningService.STOP)); });
        }
        @android.webkit.JavascriptInterface public void notificationSettings() {
            runOnUiThread(() -> startActivity(new Intent(android.provider.Settings.ACTION_APP_NOTIFICATION_SETTINGS).putExtra(android.provider.Settings.EXTRA_APP_PACKAGE, getPackageName())));
        }
        @android.webkit.JavascriptInterface public void batterySettings() {
            runOnUiThread(() -> { try { startActivity(new Intent(android.provider.Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS, Uri.parse("package:" + getPackageName()))); }
            catch (Exception error) { startActivity(new Intent(android.provider.Settings.ACTION_APPLICATION_DETAILS_SETTINGS, Uri.parse("package:" + getPackageName()))); } });
        }
    }
    private boolean local(Uri uri) {
        return origin != null && origin.equals(uri.getScheme() + "://" + uri.getAuthority());
    }
    private void toast(String message) { Toast.makeText(this, message, Toast.LENGTH_SHORT).show(); }

    @Override protected void onSaveInstanceState(Bundle state) {
        super.onSaveInstanceState(state);
        state.putString("pendingImage", pendingImage);
    }
    @Override protected void onActivityResult(int request, int result, Intent data) {
        super.onActivityResult(request, result, data);
        if (request != SAVE_IMAGE) return;
        String image = pendingImage;
        pendingImage = null;
        if (result != RESULT_OK || data == null || data.getData() == null || image == null || origin == null) return;
        Uri destination = data.getData();
        String source = origin + image;
        new Thread(() -> {
            HttpURLConnection connection = null;
            try {
                connection = (HttpURLConnection) new URL(source).openConnection();
                connection.setConnectTimeout(10000);
                connection.setReadTimeout(10000);
                connection.setInstanceFollowRedirects(false);
                if (connection.getResponseCode() != 200) throw new java.io.IOException("Image unavailable");
                try (InputStream input = connection.getInputStream(); OutputStream output = getContentResolver().openOutputStream(destination)) {
                    if (output == null) throw new java.io.IOException("Cannot open destination");
                    byte[] buffer = new byte[8192];
                    int count;
                    while ((count = input.read(buffer)) != -1) output.write(buffer, 0, count);
                }
                runOnUiThread(() -> toast("图片已保存"));
            } catch (java.io.IOException error) {
                runOnUiThread(() -> toast("图片保存失败，请重试"));
            } finally {
                if (connection != null) connection.disconnect();
            }
        }, "tiantian-save-image").start();
    }
    @Override public void onBackPressed() {
        if (webView != null && webView.canGoBack()) webView.goBack(); else super.onBackPressed();
    }
    @Override protected void onDestroy() {
        if (webView != null) { webView.stopLoading(); webView.destroy(); webView = null; }
        super.onDestroy();
    }
}
