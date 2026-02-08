import { useEffect } from 'react';
import { Phone, PhoneOff, Video, VideoOff, X } from 'lucide-react';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';

/**
 * Modal shown when receiving an incoming call
 */
export function IncomingCallModal({
  isOpen,
  caller,
  callType = 'video',
  isGroup = false,
  conversationName,
  onAccept,
  onDecline
}) {
  // Play ringtone when call comes in
  useEffect(() => {
    if (isOpen) {
      // Create and play ringtone (use a simple oscillator as placeholder)
      const audioContext = new (window.AudioContext || window.webkitAudioContext)();
      let oscillator = null;
      let gainNode = null;
      
      const playRing = () => {
        oscillator = audioContext.createOscillator();
        gainNode = audioContext.createGain();
        
        oscillator.type = 'sine';
        oscillator.frequency.setValueAtTime(440, audioContext.currentTime);
        oscillator.connect(gainNode);
        gainNode.connect(audioContext.destination);
        
        // Fade in and out for ring effect
        gainNode.gain.setValueAtTime(0, audioContext.currentTime);
        gainNode.gain.linearRampToValueAtTime(0.3, audioContext.currentTime + 0.1);
        gainNode.gain.linearRampToValueAtTime(0, audioContext.currentTime + 0.5);
        
        oscillator.start(audioContext.currentTime);
        oscillator.stop(audioContext.currentTime + 0.5);
      };

      // Ring repeatedly
      playRing();
      const interval = setInterval(playRing, 1500);

      return () => {
        clearInterval(interval);
        audioContext.close();
      };
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const displayName = isGroup ? conversationName : caller?.username;
  const avatarUrl = caller?.avatar_url;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/70 backdrop-blur-sm" />

      {/* Modal */}
      <div className="relative bg-card rounded-3xl p-8 shadow-2xl max-w-sm w-full mx-4 border border-border animate-in fade-in zoom-in duration-300">
        {/* Close button */}
        <button
          onClick={onDecline}
          className="absolute top-4 right-4 p-2 rounded-full text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
        >
          <X className="w-5 h-5" />
        </button>

        {/* Call type indicator */}
        <div className="flex justify-center mb-4">
          <div className="px-4 py-1.5 bg-primary/10 text-primary rounded-full text-sm font-medium flex items-center gap-2">
            {callType === 'video' ? (
              <Video className="w-4 h-4" />
            ) : (
              <Phone className="w-4 h-4" />
            )}
            Incoming {callType} call
          </div>
        </div>

        {/* Caller info */}
        <div className="flex flex-col items-center text-center mb-8">
          <div className="relative mb-4">
            <Avatar className="w-24 h-24 ring-4 ring-primary/20">
              <AvatarImage src={avatarUrl} alt={displayName} />
              <AvatarFallback className="bg-primary text-primary-foreground text-2xl">
                {displayName?.slice(0, 2).toUpperCase() || 'U'}
              </AvatarFallback>
            </Avatar>
            {/* Pulsing ring animation */}
            <div className="absolute inset-0 rounded-full ring-2 ring-primary animate-ping opacity-30" />
          </div>
          
          <h2 className="text-xl font-semibold text-foreground mb-1">{displayName}</h2>
          {isGroup && caller && (
            <p className="text-sm text-muted-foreground">
              {caller.username} is calling
            </p>
          )}
          <p className="text-sm text-muted-foreground animate-pulse mt-2">
            Ringing...
          </p>
        </div>

        {/* Action buttons */}
        <div className="flex items-center justify-center gap-6">
          {/* Decline button */}
          <button
            onClick={onDecline}
            className="flex flex-col items-center gap-2 group"
          >
            <div className="w-16 h-16 rounded-full bg-red-500 hover:bg-red-600 flex items-center justify-center transition-all shadow-lg group-hover:scale-105">
              <PhoneOff className="w-7 h-7 text-white" />
            </div>
            <span className="text-sm text-muted-foreground">Decline</span>
          </button>

          {/* Accept button */}
          <button
            onClick={() => onAccept(callType === 'video')}
            className="flex flex-col items-center gap-2 group"
          >
            <div className="w-16 h-16 rounded-full bg-green-500 hover:bg-green-600 flex items-center justify-center transition-all shadow-lg group-hover:scale-105 animate-bounce">
              {callType === 'video' ? (
                <Video className="w-7 h-7 text-white" />
              ) : (
                <Phone className="w-7 h-7 text-white" />
              )}
            </div>
            <span className="text-sm text-muted-foreground">Accept</span>
          </button>
        </div>
      </div>
    </div>
  );
}
