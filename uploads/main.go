te([]byte(input + "\n"))
			if err != nil {
				fmt.Println("Error sending message:", err)
				return
			}
		}
	}
}
