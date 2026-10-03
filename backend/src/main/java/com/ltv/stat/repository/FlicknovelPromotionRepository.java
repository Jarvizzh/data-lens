package com.ltv.stat.repository;

import com.ltv.stat.entity.FlicknovelPromotion;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
public interface FlicknovelPromotionRepository extends JpaRepository<FlicknovelPromotion, Long> {

    Optional<FlicknovelPromotion> findByPromotionId(String promotionId);

    List<FlicknovelPromotion> findByPromotionIdIn(List<String> promotionIds);

    List<FlicknovelPromotion> findByRechargeTplId(String rechargeTplId);

    boolean existsByPromotionId(String promotionId);

    @org.springframework.data.jpa.repository.Query("SELECT DISTINCT p.promotionId FROM FlicknovelPromotion p WHERE p.promotionId IS NOT NULL AND p.promotionId != ''")
    List<String> findAllPromotionIds();
}
