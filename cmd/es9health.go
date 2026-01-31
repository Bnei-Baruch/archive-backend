package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/Bnei-Baruch/archive-backend/es9/common"
)

var es9healthCmd = &cobra.Command{
	Use:   "es9health",
	Short: "Check Elasticsearch 9 cluster health",
	Long:  `Connects to Elasticsearch 9 and displays cluster health information`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("Starting ES9 health check")

		// Get ES9 URL from config
		url := viper.GetString("elasticsearch9.url")
		if url == "" {
			log.Fatal("elasticsearch9.url not configured")
		}

		log.Infof("Connecting to ES9 at: %s", url)

		// Create ES9 manager
		manager := common.MakeES9Manager(url)
		defer manager.Stop()

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Perform ping
		log.Info("Pinging ES9...")
		if err := manager.Ping(ctx); err != nil {
			log.Fatalf("Failed to ping ES9: %v", err)
		}
		log.Info("✓ Ping successful")

		// Get health info
		log.Info("Getting cluster health...")
		health, err := manager.HealthCheck(ctx)
		if err != nil {
			log.Fatalf("Failed to get cluster health: %v", err)
		}

		// Display health info
		fmt.Println("\n" + strings.Repeat("=", 60))
		fmt.Println("  Elasticsearch 9 Cluster Health")
		fmt.Println(strings.Repeat("=", 60))
		fmt.Printf("Cluster Name:    %s\n", health.ClusterName)
		fmt.Printf("Status:          %s\n", health.Status)
		fmt.Printf("Version:         %s\n", health.Version)
		fmt.Printf("Number of Nodes: %d\n", health.NumberOfNodes)
		fmt.Printf("Active Shards:   %d\n", health.ActiveShards)
		fmt.Printf("URL:             %s\n", url)
		fmt.Printf("Checked at:      %s\n", health.Timestamp.Format(time.RFC3339))
		fmt.Println(strings.Repeat("=", 60))

		// Status indicator
		statusEmoji := map[string]string{
			"green":  "✓",
			"yellow": "⚠",
			"red":    "✗",
		}
		emoji := statusEmoji[health.Status]
		if emoji == "" {
			emoji = "?"
		}
		fmt.Printf("\n%s Cluster status: %s\n\n", emoji, health.Status)

		log.Info("ES9 health check completed successfully")
	},
}

func init() {
	RootCmd.AddCommand(es9healthCmd)
}
