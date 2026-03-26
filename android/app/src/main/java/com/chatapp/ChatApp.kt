package com.chatapp

import android.app.Application
import dagger.hilt.android.HiltAndroidApp
import timber.log.Timber

@HiltAndroidApp
class ChatApp : Application() {

    override fun onCreate() {
        super.onCreate()
        
        // Initialize Timber for logging
        if (BuildConfig.DEBUG) {
            Timber.plant(Timber.DebugTree())
        }
        
        // Initialize crash reporting
        initializeCrashReporting()
        
        // Initialize WorkManager for background sync
        initializeWorkManager()
    }
    
    private fun initializeCrashReporting() {
        // Firebase Crashlytics initialization
        // Fabric.with(this, Crashlytics())
    }
    
    private fun initializeWorkManager() {
        // Background sync initialization
        // SyncManager.scheduleSync()
    }
}
