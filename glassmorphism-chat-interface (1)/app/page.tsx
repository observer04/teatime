"use client"

import { useState, useCallback } from "react"
import { IconSidebar } from "@/components/icon-sidebar"
import { ChatSidebar } from "@/components/chat-sidebar"
import { ChatHeader } from "@/components/chat-header"
import { ChatMessages, type Message } from "@/components/chat-messages"
import { MessageInput } from "@/components/message-input"

const initialMessages: Message[] = [
  {
    id: "1",
    sender: { id: "vipul", name: "Vipul Iiit Cse" },
    content: "Also thanks laane ke liye",
    timestamp: new Date(Date.now() - 1000 * 60 * 50),
    isOwn: false,
  },
  {
    id: "2",
    sender: { id: "me", name: "John Doe" },
    content: "Bhai khana milega?",
    timestamp: new Date(Date.now() - 1000 * 60 * 46),
    isOwn: true,
    isRead: true,
  },
  {
    id: "3",
    sender: { id: "me", name: "John Doe" },
    content: "Iss bar de de yaar phir kabhi kuch nhi bolunga laane ko",
    timestamp: new Date(Date.now() - 1000 * 60 * 44),
    isOwn: true,
    isRead: true,
  },
  {
    id: "4",
    sender: { id: "vipul", name: "Vipul Iiit Cse" },
    content: "u be",
    timestamp: new Date(Date.now() - 1000 * 60 * 43),
    isOwn: false,
  },
  {
    id: "5",
    sender: { id: "vipul", name: "Vipul Iiit Cse" },
    content: "n wle hi late kardiye isme mai kya karu",
    timestamp: new Date(Date.now() - 1000 * 60 * 43),
    isOwn: false,
  },
  {
    id: "6",
    sender: { id: "vipul", name: "Vipul Iiit Cse" },
    content: "P block ghus gye h",
    timestamp: new Date(Date.now() - 1000 * 60 * 42),
    isOwn: false,
  },
  {
    id: "7",
    sender: { id: "vipul", name: "Vipul Iiit Cse" },
    content: "2 min me pauch rhe h",
    timestamp: new Date(Date.now() - 1000 * 60 * 42),
    isOwn: false,
  },
]

const chatData: Record<string, { name: string; status?: string; avatar?: string; isChannel?: boolean; memberCount?: number }> = {
  vipul: { name: "Vipul Iiit Cse", status: "online", avatar: "/avatars/vipul.jpg" },
  self: { name: "Hello Self (You)", status: "online" },
  friend1: { name: "F", status: "offline" },
  vipu2: { name: "Vipul Iiit Cse", status: "online" },
  hari: { name: "Hari CSE IIIT", status: "offline" },
  sarah: { name: "Sarah Chen", status: "online", avatar: "/avatars/sarah.jpg" },
}

export default function ChatPage() {
  const [activeTab, setActiveTab] = useState("chats")
  const [activeChat, setActiveChat] = useState("vipul")
  const [messages, setMessages] = useState<Message[]>(initialMessages)

  const handleSendMessage = useCallback((content: string) => {
    const newMessage: Message = {
      id: Date.now().toString(),
      sender: { id: "me", name: "John Doe" },
      content,
      timestamp: new Date(),
      isOwn: true,
      isRead: false,
    }
    setMessages((prev) => [...prev, newMessage])
  }, [])

  const currentChat = chatData[activeChat] || { name: "Unknown", status: "offline" }

  return (
    <div className="flex h-screen bg-background overflow-hidden">
      {/* Left Icon Sidebar */}
      <IconSidebar activeTab={activeTab} onTabChange={setActiveTab} />

      {/* Chat List Panel */}
      <ChatSidebar activeChat={activeChat} onChatSelect={setActiveChat} />

      {/* Main Chat Area */}
      <main className="flex-1 flex flex-col min-w-0">
        <ChatHeader
          name={currentChat.name}
          status={currentChat.status}
          avatar={currentChat.avatar}
          isChannel={currentChat.isChannel}
          memberCount={currentChat.memberCount}
        />
        <ChatMessages messages={messages} />
        <MessageInput onSend={handleSendMessage} />
      </main>
    </div>
  )
}
