package com.tiantianxiangshang.app;

import android.app.*;
import android.content.*;
import android.content.pm.ServiceInfo;
import android.os.*;
import org.json.JSONObject;
import com.chaquo.python.Python;
import com.chaquo.python.android.AndroidPlatform;

public final class LearningService extends Service {
    static final String CHANNEL = "learning", STOP = "stop";
    static volatile String activeTask = "";
    private volatile boolean stopped;
    private String runID = "", taskName = "学习任务";
    private PowerManager.WakeLock wakeLock;
    private long lastNotification;
    private final Handler main = new Handler(Looper.getMainLooper());
    @Override public IBinder onBind(Intent intent) { return null; }
    @Override public int onStartCommand(Intent intent, int flags, int startId) {
        if (intent == null) { stopSelf(); return START_NOT_STICKY; }
        if (STOP.equals(intent.getAction())) { stopped = true; return START_NOT_STICKY; }
        String id = intent.getStringExtra("task_id");
        if (id == null || !id.matches("[a-f0-9]{32}")) { stopSelf(); return START_NOT_STICKY; }
        if (!activeTask.isEmpty()) { android.widget.Toast.makeText(this, "已有任务在执行，请先停止当前任务", 0).show(); return START_NOT_STICKY; }
        activeTask = id; stopped = false;
        getSystemService(NotificationManager.class).createNotificationChannel(new NotificationChannel(CHANNEL, "学习任务运行", NotificationManager.IMPORTANCE_LOW));
        if (Build.VERSION.SDK_INT >= 29) startForeground(101, notification("正在启动手机本地执行器"), ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC);
        else startForeground(101, notification("正在启动手机本地执行器"));
        wakeLock = ((PowerManager) getSystemService(POWER_SERVICE)).newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "tiantian:learning");
        wakeLock.acquire(6 * 60 * 60 * 1000L);
        new Thread(() -> execute(id), "tiantian-learning").start();
        return START_NOT_STICKY;
    }
    private Notification notification(String text) {
        PendingIntent open = PendingIntent.getActivity(this, 0, new Intent(this, MainActivity.class), PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
        PendingIntent stop = PendingIntent.getService(this, 1, new Intent(this, LearningService.class).setAction(STOP), PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
        return new Notification.Builder(this, CHANNEL).setSmallIcon(R.drawable.ic_notification).setContentTitle("天天向上 · " + taskName)
            .setContentText(text).setContentIntent(open).setOngoing(true).setOnlyAlertOnce(true)
            .addAction(new Notification.Action.Builder(null, "停止任务", stop).build()).build();
    }
    private void execute(String id) {
        try {
            JSONObject config = NativeBackend.post(this, "/api/native/tasks/" + id + "/claim", new JSONObject());
            runID = config.getString("run_id"); taskName = config.getJSONObject("task").getString("name");
            if (!Python.isStarted()) Python.start(new AndroidPlatform(this));
            Python.getInstance().getModule("mobile_runner").callAttr("run", config.toString(), getFilesDir().getAbsolutePath(), new Progress(id));
        } catch (Throwable error) {
            // Do not log exceptions containing decrypted configuration or platform URLs.
            try {
                if (!runID.isEmpty()) send(id, new JSONObject().put("state", stopped ? "cancelled" : "failed").put("message", "本机执行器启动失败，请重新打开应用后重试"));
                else main.post(() -> android.widget.Toast.makeText(this, "任务无法启动，请检查任务配置或当前运行状态", 1).show());
            } catch (Exception ignored) { }
        } finally {
            main.post(() -> { if (wakeLock != null && wakeLock.isHeld()) wakeLock.release(); stopForeground(STOP_FOREGROUND_REMOVE); activeTask = ""; stopSelf(); });
        }
    }
    private void send(String id, JSONObject event) throws Exception {
        NativeBackend.post(this, "/api/native/tasks/" + id + "/progress", new JSONObject().put("run_id", runID).put("event", event));
        if (SystemClock.elapsedRealtime() - lastNotification > 4000) {
            lastNotification = SystemClock.elapsedRealtime();
            getSystemService(NotificationManager.class).notify(101, notification(event.optString("message", "正在执行学习任务")));
        }
    }
    public final class Progress {
        private final String id;
        Progress(String id) { this.id = id; }
        public boolean isStopped() { return stopped; }
        public void report(String event) throws Exception { send(id, new JSONObject(event)); }
    }
    @Override public void onTimeout(int startId, int type) { stopped = true; stopForeground(STOP_FOREGROUND_REMOVE); stopSelf(); }
    @Override public void onDestroy() { stopped = true; if (wakeLock != null && wakeLock.isHeld()) wakeLock.release(); super.onDestroy(); }
}
